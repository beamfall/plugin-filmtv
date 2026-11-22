package filmtv

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
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
	kindFields := map[string]map[string]string{}
	relations := map[string]sdk.RelationSpec{}
	for _, kind := range bundle.EntityKinds {
		kindFields[kind.Name] = map[string]string{}
		for _, spec := range kind.Fields {
			kindFields[kind.Name][spec.Name] = spec.Type
		}
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
		check(name+" season", kindFields["season"], got.GroupFields)
		check(name+" show", kindFields["show"], got.ParentFields)
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
		t.Fatal("signature empty with sidecars present")
	}
	if again := NFOSignature(media); again != initial {
		t.Fatalf("signature unstable: %q then %q", initial, again)
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
			return os.WriteFile(filepath.Join(dir, "season.nfo"), []byte("<season><plot>Longer plot now.</plot></season>"), 0o600)
		}},
		{"removed", func() error { return os.Remove(episodeNFO) }},
		{"show removed", func() error { return os.Remove(filepath.Join(filepath.Dir(dir), "tvshow.nfo")) }},
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
		previous = current
	}
	if err := os.Remove(filepath.Join(dir, "season.nfo")); err != nil {
		t.Fatal(err)
	}
	if got := NFOSignature(media); got != "" {
		t.Fatalf("signature with no sidecars = %q, want empty", got)
	}
	if got := LocalNFOHydration(media); got.Found || len(got.GroupFields)+len(got.ParentFields) != 0 {
		t.Fatalf("hydration with no sidecars = %+v", got)
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

func TestNormalizeCertification(t *testing.T) {
	for in, want := range map[string]string{
		"Rated PG-13":    "PG-13",
		"US:R / UK:15":   "R",
		"TV-14":          "TV-14",
		" ":              "",
		"Germany:FSK 12": "Germany:FSK 12",
	} {
		if got := normalizeCertification(in); got != want {
			t.Errorf("normalizeCertification(%q) = %q, want %q", in, got, want)
		}
	}
}
