package filmtv_test

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestTVLibraryFilesNeverBecomeFilms pins the Tower TV library files that
// were listed on its Movies shelf (CORE-2313 follow-up): every file under a
// TV library folder is an episode of its show folder or, failing that, keeps
// no class. It is never a film.
func TestTVLibraryFilesNeverBecomeFilms(t *testing.T) {
	parser := filmtv.New()
	cases := []struct {
		name, path string
		want       sdk.ParsedMedia
	}{
		{"bare-e-code-in-show-folder", "/mnt/user/tv/The IT Crowd/1e01-Yesterday's Jam.mkv",
			sdk.ParsedMedia{Title: "The IT Crowd", Class: "episode", Sequence: "1", Index: 1}},
		{"leading-pair-in-named-series-folder", "/mnt/user/tv/The IT Crowd/The IT Crowd  Series 3/3.01 From Hell.mkv",
			sdk.ParsedMedia{Title: "The IT Crowd", Class: "episode", Sequence: "3", Index: 1}},
		{"leading-pair-in-show-folder", "/mnt/user/tv/The IT Crowd/3 01 From Hell.mkv",
			sdk.ParsedMedia{Title: "The IT Crowd", Class: "episode", Sequence: "3", Index: 1}},
		{"season-x-in-show-folder", "/mnt/user/tv/The IT Crowd/3x01 From Hell.mkv",
			sdk.ParsedMedia{Title: "The IT Crowd", Class: "episode", Sequence: "3", Index: 1}},
		{"series-code-after-release-group", "/mnt/user/tv/The Simpsons/aaf-simpsons2001-1080p.mkv",
			sdk.ParsedMedia{Title: "The Simpsons", Class: "episode", Sequence: "20", Index: 1}},
		{"season-extra-outtakes", "/mnt/user/tv/The IT Crowd/The IT Crowd  Series 3/3.x Out Takes.mkv",
			sdk.ParsedMedia{Title: "Out Takes"}},
		{"season-extra-deleted-scene", "/mnt/user/tv/The IT Crowd/The IT Crowd  Series 3/3.x Deleted Scene.mkv",
			sdk.ParsedMedia{Title: "Deleted Scene"}},
		{"season-extra-set-tour", "/mnt/user/tv/The IT Crowd/The IT Crowd  Series 3/3.x Set Tour.mkv",
			sdk.ParsedMedia{Title: "Set Tour"}},
		{"series-folder-under-library-root", "/mnt/user/tv/Sherlock Series 3/3.01 The Empty Hearse.mkv",
			sdk.ParsedMedia{Title: "Sherlock", Class: "episode", Sequence: "3", Index: 1}},
		{"loose-file-under-tv", "/mnt/user/tv/Some Special.mkv", sdk.ParsedMedia{Title: "Some Special"}},
		// Outside a TV library the same names keep their film reading.
		{"leading-pair-outside-tv", "/media/Movies/3 10 to Yuma.mkv",
			sdk.ParsedMedia{Title: "3 10 to Yuma", Class: "movie"}},
		{"series-code-outside-tv", "/media/Movies/Space Odyssey/space odyssey2001.mkv",
			sdk.ParsedMedia{Title: "space odyssey2001", Class: "movie"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parser.Parse(tc.path, sdk.ParseProbe{})
			if !ok {
				t.Fatalf("Parse(%q) ok = false", tc.path)
			}
			got.ProviderIDs, got.HintSource = nil, ""
			if got.Title != tc.want.Title || got.Class != tc.want.Class || got.Year != tc.want.Year ||
				got.Sequence != tc.want.Sequence || got.Index != tc.want.Index {
				t.Fatalf("Parse(%q) = %+v, want %+v", tc.path, got, tc.want)
			}
		})
	}
}
