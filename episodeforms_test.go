package filmtv_test

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestEpisodeFormsDoNotMisreadFilms pins the guards on the TV name forms added
// for CORE-2313: each form reads only where a film or a checksum cannot look
// the same.
func TestEpisodeFormsDoNotMisreadFilms(t *testing.T) {
	parser := filmtv.New()
	cases := []struct {
		name, path string
		want       sdk.ParsedMedia
	}{
		// A checksum tag holds "4E01"; a bare-e code needs a separator before
		// it and no letter or digit after it.
		{"checksum-is-not-a-season-code", "/media/Anime/Show - 01 [4E01A2B3].mkv",
			sdk.ParsedMedia{Title: "Show - 01 [4E01A2B3]", Class: "movie"}},
		// Two leading numbers outside a season folder are a film title.
		{"leading-numbers-outside-season-folder", "/media/Movies/3 10 to Yuma.mkv",
			sdk.ParsedMedia{Title: "3 10 to Yuma", Class: "movie"}},
		// A season folder that names another season does not agree.
		{"leading-numbers-disagree-with-folder", "/media/TV/Synthetic Show/Season 2/3 01 From Hell.mkv",
			sdk.ParsedMedia{Title: "3 01 From Hell", Class: "movie"}},
		// A trailing year is a year, never an absolute number.
		{"trailing-year-is-not-absolute", "/media/Movies/Blade Runner 2049.mkv",
			sdk.ParsedMedia{Title: "Blade Runner", Class: "movie", Year: 2049}},
		// A bare three-digit number without " - " stays in the title.
		{"bare-three-digits-stay-in-title", "/media/Movies/Agent 007.mkv",
			sdk.ParsedMedia{Title: "Agent 007", Class: "movie"}},
		// Absolute number zero is no episode.
		{"absolute-zero-is-not-an-episode", "/media/TV/Synthetic Show 0000.mkv",
			sdk.ParsedMedia{Title: "Synthetic Show 0000", Class: "movie"}},
		// "Specials" is season 0 and agrees with a leading 0.
		{"leading-numbers-in-specials", "/media/TV/Synthetic Show/Specials/0 02 Behind the Scenes.mkv",
			sdk.ParsedMedia{Title: "Synthetic Show", Class: "episode", Sequence: "0", Index: 2}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := parser.Parse(tc.path, sdk.ParseProbe{})
			if !ok {
				t.Fatalf("Parse(%q) refused, want %+v", tc.path, tc.want)
			}
			if got.Title != tc.want.Title || got.Class != tc.want.Class || got.Year != tc.want.Year ||
				got.Sequence != tc.want.Sequence || got.Index != tc.want.Index || got.AbsoluteOrdinal != tc.want.AbsoluteOrdinal {
				t.Errorf("Parse(%q) = %+v, want %+v", tc.path, got, tc.want)
			}
		})
	}
}
