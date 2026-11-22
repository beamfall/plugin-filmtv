package filmtv

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

var updateNFOGolden = flag.Bool("update-nfo-golden", false, "rewrite testdata/nfo/golden from LocalNFOHydration")

// nfoLayout is one library layout: the video file to hydrate and the fixture
// copied to each sidecar path, all relative to a temporary root.
type nfoLayout struct {
	media    string
	sidecars map[string]string
}

var nfoGoldenLayouts = map[string]nfoLayout{
	"kodi-film": {
		media:    "The Lantern Keeper (2019)/The Lantern Keeper (2019).mkv",
		sidecars: map[string]string{"The Lantern Keeper (2019)/The Lantern Keeper (2019).nfo": "kodi-film.nfo"},
	},
	"folder-film": {
		media:    "The Lantern Keeper (2019)/lantern.keeper.2019.mkv",
		sidecars: map[string]string{"The Lantern Keeper (2019)/" + nfoFilmRoot + ".nfo": "kodi-film.nfo"},
	},
	"episode": {
		media: "Tidewater/Season 1/Tidewater S01E01.mkv",
		sidecars: map[string]string{
			"Tidewater/tvshow.nfo":                    "tvshow.nfo",
			"Tidewater/Season 1/season.nfo":           "season.nfo",
			"Tidewater/Season 1/Tidewater S01E01.nfo": "episodedetails.nfo",
		},
	},
	"multi-episode": {
		media: "Tidewater/Season 1/Tidewater S01E02-E03.mkv",
		sidecars: map[string]string{
			"Tidewater/tvshow.nfo":                        "tvshow.nfo",
			"Tidewater/Season 1/Tidewater S01E02-E03.nfo": "multi-episode.nfo",
		},
	},
	"show-only": {
		media:    "Tidewater/Season 1/Tidewater S01E04.mkv",
		sidecars: map[string]string{"Tidewater/tvshow.nfo": "tvshow.nfo"},
	},
	"latin1": {
		media:    "Cafe Melodie (2015)/Cafe Melodie (2015).mkv",
		sidecars: map[string]string{"Cafe Melodie (2015)/Cafe Melodie (2015).nfo": "latin1.nfo"},
	},
	"truncated": {
		media:    "Salt Roads (2018)/Salt Roads (2018).mkv",
		sidecars: map[string]string{"Salt Roads (2018)/Salt Roads (2018).nfo": "truncated.nfo"},
	},
	"none": {media: "Bare (2001)/Bare (2001).mkv"},
}

// materialize writes layout under a fresh root and returns the media path.
func (l nfoLayout) materialize(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rel string, data []byte) {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write(l.media, nil)
	for rel, fixture := range l.sidecars {
		data, err := os.ReadFile(filepath.Join("testdata", "nfo", fixture))
		if err != nil {
			t.Fatal(err)
		}
		write(rel, data)
	}
	return filepath.Join(root, filepath.FromSlash(l.media))
}

func TestLocalNFOHydrationGolden(t *testing.T) {
	for name, layout := range nfoGoldenLayouts {
		t.Run(name, func(t *testing.T) {
			got, err := json.MarshalIndent(LocalNFOHydration(layout.materialize(t)), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, '\n')
			golden := filepath.Join("testdata", "nfo", "golden", name+".json")
			if *updateNFOGolden {
				if err := os.MkdirAll(filepath.Dir(golden), 0o750); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(golden, got, 0o600); err != nil {
					t.Fatal(err)
				}
				return
			}
			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatalf("read golden (run with -update-nfo-golden): %v", err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("LocalNFOHydration mismatch for %s\ngot:\n%s\nwant:\n%s", name, got, want)
			}
		})
	}
}

// TestLocalNFOHydrationFieldsAreDeclared keeps every written field and
// relation inside the bundle's declarations, with matching types.
func TestLocalNFOHydrationFieldsAreDeclared(t *testing.T) {
	bundle := Bundle()
	mediaTypes := map[string]string{}
	for _, spec := range bundle.MediaFields {
		mediaTypes[spec.Name] = spec.Type
	}
	relations := map[string]sdk.RelationSpec{}
	for _, kind := range bundle.EntityKinds {
		for _, relation := range kind.Relations {
			relations[kind.Name+"\x00"+relation.Name] = relation
		}
	}
	if mediaTypes[NFOSignatureField] != "text" {
		t.Fatalf("%s type = %q, want text", NFOSignatureField, mediaTypes[NFOSignatureField])
	}
	check := func(scope string, declared map[string]string, fields []SidecarField) {
		for _, field := range fields {
			if declared[field.Name] != field.Type {
				t.Errorf("%s field %s type %q, declared %q", scope, field.Name, field.Type, declared[field.Name])
			}
		}
	}
	for name, layout := range nfoGoldenLayouts {
		got := LocalNFOHydration(layout.materialize(t))
		check(name+" media", mediaTypes, got.MediaFields)
		for _, relation := range got.Relations {
			spec, ok := relations[relation.FromKind+"\x00"+relation.Name]
			if !ok || spec.ToKind != sdk.MediaKindName {
				t.Errorf("%s relation %s/%s is not a declared Media relation", name, relation.FromKind, relation.Name)
				continue
			}
			for attribute := range relation.Attributes {
				found := false
				for _, declared := range spec.Attributes {
					found = found || declared == attribute
				}
				if !found {
					t.Errorf("%s relation %s attribute %s undeclared", name, relation.Name, attribute)
				}
			}
		}
	}
}

func TestNFOSignatureTracksSidecars(t *testing.T) {
	media := nfoGoldenLayouts["episode"].materialize(t)
	dir := filepath.Dir(media)
	initial := NFOSignature(media)
	if initial == "" {
		t.Fatal("signature empty with a sidecar present")
	}
	if again := NFOSignature(media); again != initial {
		t.Fatalf("signature unstable: %q then %q", initial, again)
	}
	if got := LocalNFOHydration(media).Signature; got != initial {
		t.Fatalf("plan signature %q, want the stat signature %q", got, initial)
	}
	episodeNFO := filepath.Join(dir, "Tidewater S01E01.nfo")
	steps := []struct {
		name   string
		change func() error
	}{
		{"mtime", func() error {
			future := time.Now().Add(time.Hour)
			return os.Chtimes(episodeNFO, future, future)
		}},
		{"size", func() error {
			return os.WriteFile(episodeNFO, []byte("<episodedetails><plot>Longer plot now.</plot></episodedetails>"), 0o600)
		}},
		{"removed", func() error { return os.Remove(episodeNFO) }},
	}
	previous := initial
	for _, step := range steps {
		if err := step.change(); err != nil {
			t.Fatal(err)
		}
		current := NFOSignature(media)
		if current == previous {
			t.Fatalf("%s: signature unchanged (%q)", step.name, current)
		}
		if got := LocalNFOHydration(media).Signature; got != current {
			t.Fatalf("%s: plan signature %q, want %q", step.name, got, current)
		}
		previous = current
	}
	if previous != "" {
		t.Fatalf("signature with no media NFO = %q, want empty (container sidecars are not read)", previous)
	}
	if got := LocalNFOHydration(media); got.Found || got.Unreadable {
		t.Fatalf("hydration with only container sidecars = %+v", got)
	}
}

// TestNFOSignatureNamesTheSidecar: a per-file NFO and the folder film NFO with
// the same size and mtime still sign differently, so switching which one
// feeds the plan triggers a rescan.
func TestNFOSignatureNamesTheSidecar(t *testing.T) {
	media := nfoGoldenLayouts["folder-film"].materialize(t)
	folder := filepath.Join(filepath.Dir(media), nfoFilmRoot+".nfo")
	before := NFOSignature(media)
	info, err := os.Stat(folder)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(folder)
	if err != nil {
		t.Fatal(err)
	}
	own := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	if err := os.WriteFile(own, data, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(own, info.ModTime(), info.ModTime()); err != nil {
		t.Fatal(err)
	}
	if after := NFOSignature(media); after == before {
		t.Fatalf("signature %q unchanged after a per-file NFO took over", after)
	}
}

func TestLocalNFOHydrationUnreadableIsNotRemoval(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	media := nfoGoldenLayouts["kodi-film"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	if err := os.Chmod(sidecar, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(sidecar, 0o600) })
	got := LocalNFOHydration(media)
	if !got.Unreadable || got.Found || got.Signature != "" || got.Title != "" {
		t.Fatalf("unreadable sidecar plan = %+v, want Unreadable with no values or signature", got)
	}
	if err := os.Chmod(sidecar, 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LocalNFOHydration(media); got.Unreadable || !got.Found || got.Signature != NFOSignature(media) {
		t.Fatalf("restored sidecar plan = %+v", got)
	}
}

func TestLocalNFOHydrationSyntaxErrorIsUnreadable(t *testing.T) {
	media := nfoGoldenLayouts["none"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	if err := os.WriteFile(sidecar, []byte("<?xml version=\"1.0\"?>\n</broken>"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LocalNFOHydration(media); !got.Unreadable {
		t.Fatalf("syntax-error sidecar plan = %+v, want Unreadable", got)
	}
	if err := os.WriteFile(sidecar, []byte("https://www.imdb.com/title/tt0000001/\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := LocalNFOHydration(media); got.Unreadable || got.Found || got.Signature == "" {
		t.Fatalf("URL-only sidecar plan = %+v, want readable, empty, signed", got)
	}
}

func TestLocalNFOHydrationRefusesAnEditDuringTheRead(t *testing.T) {
	media := nfoGoldenLayouts["kodi-film"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	for name, edit := range map[string]func() error{
		"rewritten": func() error {
			return os.WriteFile(sidecar, []byte("<"+nfoFilmRoot+"><title>New</title></"+nfoFilmRoot+">"), 0o600)
		},
		"replaced": func() error {
			next := sidecar + ".tmp"
			if err := os.WriteFile(next, []byte("<"+nfoFilmRoot+"><title>Other</title></"+nfoFilmRoot+">"), 0o600); err != nil {
				return err
			}
			return os.Rename(next, sidecar)
		},
	} {
		t.Run(name, func(t *testing.T) {
			afterNFORead = func(string) {
				if err := edit(); err != nil {
					t.Error(err)
				}
			}
			t.Cleanup(func() { afterNFORead = func(string) {} })
			if got := LocalNFOHydration(media); !got.Unreadable || got.Signature != "" {
				t.Fatalf("plan read across an edit = %+v, want Unreadable", got)
			}
		})
	}
}

func TestFolderFilmNFODescribesOnlyTheMainFeature(t *testing.T) {
	media := nfoGoldenLayouts["folder-film"].materialize(t)
	dir := filepath.Dir(media)
	for _, extra := range []string{"lantern.keeper.2019-trailer.mkv", "Behind the Scenes-featurette.mkv", "trailer.mkv", "Film - Deleted Scenes.mkv"} {
		path := filepath.Join(dir, extra)
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
		if got := LocalNFOHydration(path); got.Found || NFOSignature(path) != "" {
			t.Errorf("%s took the folder NFO: %+v", extra, got)
		}
		if got := nfoWatchState(path); got.Present {
			t.Errorf("%s took the folder NFO play: %+v", extra, got)
		}
	}
	if got := LocalNFOHydration(media); !got.Found {
		t.Fatal("main feature lost the folder NFO")
	}
}

func TestEpisodeKeepsExplicitYearAndSortTitle(t *testing.T) {
	media := nfoGoldenLayouts["none"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	doc := "<episodedetails><title>Pilot</title><year>1999</year><sorttitle>First</sorttitle></episodedetails>"
	if err := os.WriteFile(sidecar, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
	got := LocalNFOHydration(media)
	if got.Year != 1999 || got.SortTitle != "First" {
		t.Fatalf("episode year %d sort title %q, want 1999 and First", got.Year, got.SortTitle)
	}
}

func TestNFOOwnedRelationsCoverEveryPlannedRelation(t *testing.T) {
	owned := map[SidecarRelationKind]bool{}
	for _, kind := range NFOOwnedRelations() {
		owned[kind] = true
	}
	for name, layout := range nfoGoldenLayouts {
		for _, relation := range LocalNFOHydration(layout.materialize(t)).Relations {
			if !owned[SidecarRelationKind{FromKind: relation.FromKind, Name: relation.Name}] {
				t.Errorf("%s relation %s/%s is not owned", name, relation.FromKind, relation.Name)
			}
		}
	}
}

func TestNFOSourceCarriesPlayRecord(t *testing.T) {
	root := t.TempDir()
	film := nfoGoldenLayouts["kodi-film"]
	episode := nfoGoldenLayouts["episode"]
	paths := map[string]string{}
	for name, layout := range map[string]nfoLayout{"film": film, "episode": episode, "bare": nfoGoldenLayouts["none"]} {
		media := layout.materialize(t)
		target := filepath.Join(root, name, filepath.Base(media))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.Rename(media, target); err != nil {
			t.Fatal(err)
		}
		for rel := range layout.sidecars {
			src := filepath.Join(filepath.Dir(media), filepath.Base(rel))
			if filepath.Dir(filepath.FromSlash(rel)) != filepath.Dir(filepath.FromSlash(layout.media)) {
				continue
			}
			if err := os.Rename(src, filepath.Join(filepath.Dir(target), filepath.Base(rel))); err != nil {
				t.Fatal(err)
			}
		}
		paths[name] = target
	}
	items, err := NewNFOSource(root).Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]sdk.WatchState{}
	for _, item := range items {
		got[item.Path] = item.WatchState
	}
	want := map[string]sdk.WatchState{
		paths["film"]:    {Present: true, Watched: true, OccurredAt: time.Date(2025, 3, 14, 21, 5, 0, 0, time.UTC)},
		paths["episode"]: {Present: true, Watched: true},
		paths["bare"]:    {},
	}
	for path, state := range want {
		if got[path] != state {
			t.Errorf("watch state for %s = %+v, want %+v", filepath.Base(path), got[path], state)
		}
	}
}

func TestSelectCertification(t *testing.T) {
	for in, want := range map[string]struct {
		label  string
		policy bool
	}{
		"Rated PG-13":    {"PG-13", true},
		"US:R / UK:15":   {"R", true},
		"CA:PG / US:R":   {"R", true},
		"CA:PG / R":      {"R", true},
		"CA:PG":          {"PG", false},
		"US:Rated R":     {"R", true},
		"TV-14":          {"TV-14", true},
		" ":              {"", false},
		"Germany:FSK 12": {"Germany:FSK 12", true},
	} {
		label, policy := selectCertification(in)
		if label != want.label || policy != want.policy {
			t.Errorf("selectCertification(%q) = %q, %v, want %q, %v", in, label, policy, want.label, want.policy)
		}
	}
}

func TestForeignRatingNeverSetsTheCeiling(t *testing.T) {
	for mpaa, want := range map[string]string{"CA:PG / US:R": "R", "CA:PG": "", "PG-13": "PG-13"} {
		got := ""
		for _, field := range certificationFields(mpaa) {
			if field.Name == "certification" {
				got = field.Text
			}
		}
		if got != want {
			t.Errorf("certification for %q = %q, want %q", mpaa, got, want)
		}
	}
}

func TestNFOSourceHonoursCategorySelection(t *testing.T) {
	media := nfoGoldenLayouts["kodi-film"].materialize(t)
	root := filepath.Dir(filepath.Dir(media))
	for name, tc := range map[string]struct {
		requested []string
		watched   bool
	}{
		"collections only": {[]string{sdk.CategoryCollections}, false},
		"watch state":      {[]string{sdk.CategoryWatchState}, true},
		"default":          {nil, true},
	} {
		t.Run(name, func(t *testing.T) {
			source := NewNFOSourceForCategories(root, tc.requested)
			if got := slices.Contains(source.Categories(), sdk.CategoryWatchState); got != tc.watched {
				t.Fatalf("watch-state advertised = %v, want %v", got, tc.watched)
			}
			items, err := source.Items(context.Background()) //nolint:staticcheck // deliberate SDK 0.x compatibility assertion
			if err != nil {
				t.Fatal(err)
			}
			if len(items) != 1 || items[0].WatchState.Present != tc.watched {
				t.Fatalf("items = %+v, want one with watch state %v", items, tc.watched)
			}
		})
	}
}
