package filmtv

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io/fs"
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
	initial := mustNFOSignature(t, media)
	if initial == "" {
		t.Fatal("signature empty with a sidecar present")
	}
	if again := mustNFOSignature(t, media); again != initial {
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
		current := mustNFOSignature(t, media)
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
	before := mustNFOSignature(t, media)
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
	if after := mustNFOSignature(t, media); after == before {
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
	if got := LocalNFOHydration(media); got.Unreadable || !got.Found || got.Signature != mustNFOSignature(t, media) {
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
		if got := LocalNFOHydration(path); got.Found || mustNFOSignature(t, path) != "" {
			t.Errorf("%s took the folder NFO: %+v", extra, got)
		}
		if got := mustWatchState(t, path); got.Present {
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
		"Rated PG-13":                {"PG-13", true},
		"US:R / UK:15":               {"R", true},
		"CA:PG / US:R":               {"R", true},
		"CA:PG / R":                  {"R", true},
		"CA:PG":                      {"PG", false},
		"US:Rated R":                 {"R", true},
		"TV-14":                      {"TV-14", true},
		" ":                          {"", false},
		"Germany:FSK 12":             {"FSK 12", false},
		"Germany:FSK 16 / R":         {"R", true},
		"United States:PG":           {"PG", true},
		"U.S.A.:PG-13":               {"PG-13", true},
		"Côte d’Ivoire:12 / R":       {"R", true},
		"Guinea-Bissau:12 / R":       {"R", true},
		"Co\u0302te d’Ivoire:12 / R": {"R", true},
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

func mustNFOSignature(t *testing.T, media string) string {
	t.Helper()
	signature, err := NFOSignature(media)
	if err != nil {
		t.Fatalf("NFOSignature(%s): %v", media, err)
	}
	return signature
}

func mustWatchState(t *testing.T, media string) sdk.WatchState {
	t.Helper()
	state, err := nfoWatchState(media)
	if err != nil {
		t.Fatalf("nfoWatchState(%s): %v", media, err)
	}
	return state
}

func writeNFO(t *testing.T, path, doc string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(doc), 0o600); err != nil {
		t.Fatal(err)
	}
}

// TestNFOLookupErrorIsUnreadable: a sidecar behind a folder that denies
// traversal is unreadable everywhere, never absent (Codex r1).
func TestNFOLookupErrorIsUnreadable(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root traverses a mode-000 folder")
	}
	root := t.TempDir()
	media := filepath.Join(root, "Film", "Film.mkv")
	writeNFO(t, media, "")
	hidden := filepath.Join(root, "hidden")
	writeNFO(t, filepath.Join(hidden, "real.nfo"), "<"+nfoFilmRoot+"><title>Real</title><playcount>1</playcount></"+nfoFilmRoot+">")
	if err := os.Symlink(filepath.Join(hidden, "real.nfo"), filepath.Join(root, "Film", "Film.nfo")); err != nil {
		t.Fatal(err)
	}
	if got := LocalNFOHydration(media); !got.Found || got.Title != "Real" {
		t.Fatalf("reachable symlinked sidecar plan = %+v", got)
	}
	if err := os.Chmod(hidden, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(hidden, 0o750) })
	if got := LocalNFOHydration(media); !got.Unreadable || got.Signature != "" {
		t.Errorf("plan behind a denied folder = %+v, want Unreadable", got)
	}
	if signature, err := NFOSignature(media); err == nil {
		t.Errorf("NFOSignature behind a denied folder = %q, nil; want an error", signature)
	}
	if state, err := nfoWatchState(media); err == nil {
		t.Errorf("nfoWatchState behind a denied folder = %+v, nil; want an error", state)
	}
}

// TestDanglingNFOSymlinkIsUnreadable: a sidecar symlink whose target moved is
// unreadable, never a removal that would release local ownership (Codex r2).
func TestDanglingNFOSymlinkIsUnreadable(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "Film", "Film.mkv")
	writeNFO(t, media, "")
	if err := os.Symlink(filepath.Join(root, "gone.nfo"), filepath.Join(root, "Film", "Film.nfo")); err != nil {
		t.Fatal(err)
	}
	if got := LocalNFOHydration(media); !got.Unreadable {
		t.Errorf("plan through a dangling symlink = %+v, want Unreadable", got)
	}
	if signature, err := NFOSignature(media); err == nil {
		t.Errorf("NFOSignature through a dangling symlink = %q, nil; want an error", signature)
	}
	if state, err := nfoWatchState(media); err == nil {
		t.Errorf("nfoWatchState through a dangling symlink = %+v, nil; want an error", state)
	}
}

// TestOverlongSidecarNameFallsBackToFolderNFO: a media name that fits the
// filesystem while its derived .nfo name does not still finds the folder NFO
// (Codex r2).
func TestOverlongSidecarNameFallsBackToFolderNFO(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "Film", strings.Repeat("a", 252)+".ts")
	writeNFO(t, media, "")
	folder := filepath.Join(root, "Film", nfoFilmRoot+".nfo")
	writeNFO(t, folder, "<"+nfoFilmRoot+"><title>Real</title></"+nfoFilmRoot+">")
	if got, err := mediaNFOPath(media); err != nil || got != folder {
		t.Fatalf("mediaNFOPath = %q, %v; want %q", got, err, folder)
	}
}

// TestOverlongSidecarNameInUnlistableFolder: a folder that allows lookups but
// not listing still yields its folder NFO when the derived .nfo name is too
// long for any filesystem (Codex r4).
func TestOverlongSidecarNameInUnlistableFolder(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores folder permissions")
	}
	root := t.TempDir()
	dir := filepath.Join(root, "Film")
	media := filepath.Join(dir, strings.Repeat("a", 252)+".ts")
	writeNFO(t, media, "")
	folder := filepath.Join(dir, nfoFilmRoot+".nfo")
	writeNFO(t, folder, "<"+nfoFilmRoot+"><title>Real</title></"+nfoFilmRoot+">")
	if err := os.Chmod(dir, 0o300); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
	if got, err := mediaNFOPath(media); err != nil || got != folder {
		t.Fatalf("mediaNFOPath = %q, %v; want %q", got, err, folder)
	}
}

// TestCaseFoldedLoopIsUnreadable: on a filesystem that folds case, a symlink
// loop stored as Film.NFO fails the lookup of Film.nfo; that failure is an
// unreadable sidecar, not an absent one (Codex r4).
func TestCaseFoldedLoopIsUnreadable(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "Film")
	media := filepath.Join(dir, "Film.mkv")
	writeNFO(t, media, "")
	loop := filepath.Join(dir, "Film.NFO")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Lstat(filepath.Join(dir, "Film.nfo")); err != nil {
		t.Skip("filesystem does not fold case")
	}
	if got, err := mediaNFOPath(media); err == nil {
		t.Fatalf("mediaNFOPath = %q, nil; want an error", got)
	}
}

// TestFullPathLimitFallsBackToFolderNFO: a media path that fits the system's
// path limit while its derived .nfo path does not still finds the folder NFO
// (Codex r5).
func TestFullPathLimitFallsBackToFolderNFO(t *testing.T) {
	for _, limit := range []int{1024, 4096} {
		// The kernel counts the resolved path: macOS's temp dir sits under the
		// /var symlink.
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		for len(dir) < limit-120 {
			next := filepath.Join(dir, strings.Repeat("d", min(200, limit-120-len(dir))))
			if err := os.Mkdir(next, 0o700); err != nil {
				break
			}
			dir = next
		}
		n := limit - 1 - len(dir) - 1 - len(".ts")
		if n < 1 {
			continue
		}
		stem := strings.Repeat("f", n)
		media := filepath.Join(dir, stem+".ts")
		if os.WriteFile(media, nil, 0o600) != nil {
			continue
		}
		// Only a refused lookup puts the limit in play; the assertion below
		// then holds whatever the probe found.
		if _, err := os.Stat(filepath.Join(dir, stem+".nfo")); err == nil || errors.Is(err, fs.ErrNotExist) {
			continue
		}
		folder := filepath.Join(dir, nfoFilmRoot+".nfo")
		writeNFO(t, folder, "<"+nfoFilmRoot+"><title>Real</title></"+nfoFilmRoot+">")
		if got, err := mediaNFOPath(media); err != nil || got != folder {
			t.Fatalf("mediaNFOPath = %q, %v; want %q", got, err, folder)
		}
		// A folder that grants traversal but not reading cannot be searched,
		// so the sidecar is unknown, not absent (Codex r7).
		if err := os.Chmod(dir, 0o300); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })
		if _, err := os.ReadDir(dir); err == nil {
			t.Skip("folder stays listable; running as root?")
		}
		if got, err := mediaNFOPath(media); err == nil {
			t.Fatalf("unlistable mediaNFOPath = %q, nil; want an error", got)
		}
		return
	}
	t.Skip("no path limit of 1024 or 4096 bytes")
}

// TestPathAndNameLimitsTogetherFallBackToFolderNFO: a 255-byte media name
// whose path just fits, so its .nfo passes both the path and the name limit,
// still finds the folder NFO (Codex r8).
func TestPathAndNameLimitsTogetherFallBackToFolderNFO(t *testing.T) {
	for _, limit := range []int{1024, 4096} {
		dir, err := filepath.EvalSymlinks(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		want := limit - 1 - 1 - 255
		for len(dir) < want {
			n := min(200, want-len(dir)-1)
			if n < 1 {
				break
			}
			next := filepath.Join(dir, strings.Repeat("d", n))
			if err := os.Mkdir(next, 0o700); err != nil {
				break
			}
			dir = next
		}
		media := filepath.Join(dir, strings.Repeat("f", 252)+".ts")
		if len(dir) != want || os.WriteFile(media, nil, 0o600) != nil {
			continue
		}
		folder := filepath.Join(dir, nfoFilmRoot+".nfo")
		writeNFO(t, folder, "<"+nfoFilmRoot+"><title>Real</title></"+nfoFilmRoot+">")
		if got, err := mediaNFOPath(media); err != nil || got != folder {
			t.Fatalf("mediaNFOPath = %q, %v; want %q", got, err, folder)
		}
		return
	}
	t.Skip("no 255-byte name fits a path limit of 1024 or 4096 bytes")
}

// TestTooLongProbe: an over-long name reads as too long in its folder, and a
// symlink loop or a missing file does not.
func TestTooLongProbe(t *testing.T) {
	dir := t.TempDir()
	if _, err := os.Stat(filepath.Join(dir, strings.Repeat("x", 300))); !tooLong(dir, err) {
		t.Fatalf("300-byte name stat = %v, want too long", err)
	}
	loop := filepath.Join(dir, "loop.nfo")
	if err := os.Symlink(loop, loop); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(loop); err == nil || tooLong(dir, err) {
		t.Fatalf("loop stat error %v read as too long", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "absent.nfo")); tooLong(dir, err) {
		t.Fatalf("missing file read as too long")
	}
}

// TestSymlinkToTooLongTargetIsUnreadable: a sidecar symlink whose target the
// system refuses as too long is an error, not a missing sidecar (Codex r6).
func TestSymlinkToTooLongTargetIsUnreadable(t *testing.T) {
	root := t.TempDir()
	media := filepath.Join(root, "Film.mkv")
	if err := os.WriteFile(media, nil, 0o600); err != nil {
		t.Fatal(err)
	}
	writeNFO(t, filepath.Join(root, nfoFilmRoot+".nfo"), "<"+nfoFilmRoot+"><title>Folder</title></"+nfoFilmRoot+">")
	if err := os.Symlink(filepath.Join(root, strings.Repeat("t", 300)), filepath.Join(root, "Film.nfo")); err != nil {
		t.Skipf("no symlinks: %v", err)
	}
	if got, err := mediaNFOPath(media); !errors.Is(err, errNFOUnreachable) {
		t.Fatalf("mediaNFOPath = %q, %v; want %v", got, err, errNFOUnreachable)
	}
}

// TestLongMultibyteSidecarNameIsFound: a sidecar whose name passes 255 bytes
// but fits the filesystem's limit is still found (Codex r3).
func TestLongMultibyteSidecarNameIsFound(t *testing.T) {
	root := t.TempDir()
	stem := strings.Repeat("映", 90)
	media := filepath.Join(root, "Film", stem+".mkv")
	writeNFO(t, filepath.Join(root, "Film", nfoFilmRoot+".nfo"), "<"+nfoFilmRoot+"><title>Folder</title></"+nfoFilmRoot+">")
	if err := os.WriteFile(media, nil, 0o600); err != nil {
		t.Skipf("filesystem refuses a %d-byte name: %v", len(stem)+4, err)
	}
	sidecar := filepath.Join(root, "Film", stem+".nfo")
	if err := os.WriteFile(sidecar, []byte("<"+nfoFilmRoot+"><title>Long</title></"+nfoFilmRoot+">"), 0o600); err != nil {
		t.Skipf("filesystem refuses a %d-byte name: %v", len(stem)+4, err)
	}
	if got, err := mediaNFOPath(media); err != nil || got != sidecar {
		t.Fatalf("mediaNFOPath = %q, %v; want %q", got, err, sidecar)
	}
}

// TestOversizedNFOIsUnreadable: a file past the read limit is not parsed from
// its head, which would drop the credits after the limit (Codex r1).
func TestOversizedNFOIsUnreadable(t *testing.T) {
	media := nfoGoldenLayouts["none"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	doc := "<" + nfoFilmRoot + "><title>Big</title><plot>" + strings.Repeat("x", maxNFOFileBytes) +
		"</plot><actor><name>Late Actor</name></actor></" + nfoFilmRoot + ">"
	writeNFO(t, sidecar, doc)
	if got := LocalNFOHydration(media); !got.Unreadable || got.Found || got.Signature != "" {
		t.Fatalf("oversized sidecar plan Unreadable=%v Found=%v Signature=%q, want Unreadable", got.Unreadable, got.Found, got.Signature)
	}
}

// TestSpacedSampleIsAnExtra follows Jellyfin's suffix list: trailer and
// sample after a space, dot or underscore; other extras after a hyphen only.
func TestSpacedSampleIsAnExtra(t *testing.T) {
	media := nfoGoldenLayouts["folder-film"].materialize(t)
	dir := filepath.Dir(media)
	for name, extra := range map[string]bool{
		"Film sample.mkv": true, "Film.sample.mkv": true, "Film_trailer.mkv": true, "Film Trailer.mkv": true,
		"The Big Short.mkv": false, "Short Circuit.mkv": false,
	} {
		path := filepath.Join(dir, name)
		writeNFO(t, path, "")
		if got := LocalNFOHydration(path).Found; got == extra {
			t.Errorf("%s took the folder NFO = %v, want %v", name, got, !extra)
		}
		if got := mustWatchState(t, path).Present; extra && got {
			t.Errorf("%s took the folder NFO play", name)
		}
	}
}

// TestEpisodeTagsComeFromEveryRoot (Codex r1).
func TestEpisodeTagsComeFromEveryRoot(t *testing.T) {
	media := nfoGoldenLayouts["none"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	writeNFO(t, sidecar, "<episodedetails><title>One</title><tag>Opener</tag></episodedetails>"+
		"<episodedetails><title>Two</title><tag>Finale</tag><tag>opener</tag></episodedetails>")
	got := ""
	for _, field := range LocalNFOHydration(media).MediaFields {
		if field.Name == tagsField {
			got = field.Text
		}
	}
	if got != "Opener, Finale" {
		t.Fatalf("tags = %q, want both roots' tags once", got)
	}
}

// TestNonFiniteRatingKeepsTheRest: a NaN rating is no rating, and the title
// and credits still reach the plan (Codex r1).
func TestNonFiniteRatingKeepsTheRest(t *testing.T) {
	media := nfoGoldenLayouts["none"].materialize(t)
	sidecar := strings.TrimSuffix(media, filepath.Ext(media)) + ".nfo"
	for _, rating := range []string{"<ratings><rating><value>NaN</value></rating></ratings>", "<rating>Inf</rating>",
		`<ratings><rating max="1e-300"><value>1e300</value></rating></ratings>`} {
		writeNFO(t, sidecar, "<"+nfoFilmRoot+"><title>Kept</title>"+rating+"<userrating>NaN</userrating></"+nfoFilmRoot+">")
		got := LocalNFOHydration(media)
		if got.Title != "Kept" {
			t.Fatalf("%s: title %q, want Kept", rating, got.Title)
		}
		for _, field := range got.MediaFields {
			if field.Type == "number" {
				t.Errorf("%s: number field %s = %v survived", rating, field.Name, field.Number)
			}
		}
	}
}

// TestSignatureNamesTheAbsoluteSidecar: two folders' film NFOs with the same
// name, size and mtime sign differently (Codex r1).
func TestSignatureNamesTheAbsoluteSidecar(t *testing.T) {
	root := t.TempDir()
	stamp := time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)
	var signatures []string
	for _, folder := range []string{"A", "B"} {
		media := filepath.Join(root, folder, "Film.mkv")
		writeNFO(t, media, "")
		sidecar := filepath.Join(root, folder, nfoFilmRoot+".nfo")
		writeNFO(t, sidecar, "<"+nfoFilmRoot+"><title>"+folder+"</title></"+nfoFilmRoot+">")
		if err := os.Chtimes(sidecar, stamp, stamp); err != nil {
			t.Fatal(err)
		}
		signature := mustNFOSignature(t, media)
		if !strings.Contains(signature, sidecar) {
			t.Errorf("signature %q does not name %s", signature, sidecar)
		}
		signatures = append(signatures, signature)
	}
	if signatures[0] == signatures[1] {
		t.Fatalf("both folders sign %q", signatures[0])
	}
}

// TestNFOSourceFailsOnAnUnreadableNFO: a watch-state run errors instead of
// checkpointing past a play it could not read, and a resume past that item
// does not read it again (Codex r1).
func TestNFOSourceFailsOnAnUnreadableNFO(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("root reads a mode-000 file")
	}
	root := t.TempDir()
	locked := filepath.Join(root, "a", "Film.nfo")
	writeNFO(t, filepath.Join(root, "a", "Film.mkv"), "")
	writeNFO(t, locked, "<"+nfoFilmRoot+"><playcount>1</playcount></"+nfoFilmRoot+">")
	writeNFO(t, filepath.Join(root, "b", "Film.mkv"), "")
	if err := os.Chmod(locked, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o600) })
	source := nfoSource{root: root, categories: []string{sdk.CategoryWatchState}}
	if items, err := source.Items(context.Background()); err == nil {
		t.Errorf("Items over an unreadable NFO = %+v, nil; want an error", items)
	}
	emitted := 0
	emit := func(_ context.Context, items []sdk.ForeignItem, _ string) error {
		emitted += len(items)
		return nil
	}
	if err := source.Stream(context.Background(), "", emit); err == nil {
		t.Error("Stream over an unreadable NFO returned nil")
	}
	emitted = 0
	if err := source.Stream(context.Background(), "1", emit); err != nil || emitted != 1 {
		t.Errorf("resumed Stream = %v with %d items, want nil and 1 (the skipped NFO is not read)", err, emitted)
	}
}
