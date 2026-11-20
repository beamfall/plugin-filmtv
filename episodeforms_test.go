package filmtv_test

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestEpisodeFormsDoNotMisreadFilms pins the guards on the weak TV name forms
// added for CORE-2313 (a bare-e code, a leading number pair, an absolute
// number): each reads only with episode context, so a film, a camera clip or
// a checksum never looks like an episode.
func TestEpisodeFormsDoNotMisreadFilms(t *testing.T) {
	parser := filmtv.New()
	cases := []struct {
		name, path string
		want       sdk.ParsedMedia
	}{
		// A checksum tag holds "4E01"; it is release noise, never a code. The
		// file is absolute episode 1, with no season.
		{"checksum-is-not-a-season-code", "/media/Anime/Show - 01 [4E01A2B3].mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		// Two leading numbers outside a season folder are a film title.
		{"leading-numbers-outside-season-folder", "/media/Movies/3 10 to Yuma.mkv",
			sdk.ParsedMedia{Title: "3 10 to Yuma", Class: "movie"}},
		// A season folder that names another season does not agree.
		{"leading-numbers-disagree-with-folder", "/media/TV/Synthetic Show/Season 2/3 01 From Hell.mkv",
			sdk.ParsedMedia{Title: "3 01 From Hell"}},
		// A trailing year is a year, never an absolute number.
		{"trailing-year-is-not-absolute", "/media/Movies/Blade Runner 2049.mkv",
			sdk.ParsedMedia{Title: "Blade Runner", Class: "movie", Year: 2049}},
		// A bare three-digit number without " - " stays in the title.
		{"bare-three-digits-stay-in-title", "/media/Movies/Agent 007.mkv",
			sdk.ParsedMedia{Title: "Agent 007", Class: "movie"}},
		// Absolute number zero is no episode.
		{"absolute-zero-is-not-an-episode", "/media/TV/Synthetic Show 0000.mkv",
			sdk.ParsedMedia{Title: "Synthetic Show 0000"}},
		// Absolute numbers need episode context: a film or home-video name
		// with no season, show or TV folder keeps its number.
		{"four-digits-in-film-name", "Agent 0011.mkv", sdk.ParsedMedia{Title: "Agent 0011", Class: "movie"}},
		{"dash-four-digits-in-film-name", "Movie - 0420.mkv", sdk.ParsedMedia{Title: "Movie - 0420", Class: "movie"}},
		{"home-video-number", "Wedding_0002.mp4", sdk.ParsedMedia{Title: "Wedding 0002", Class: "movie"}},
		{"dash-three-digits-in-film-name", "Bond - 007.mkv", sdk.ParsedMedia{Title: "Bond - 007", Class: "movie"}},
		{"numbered-film-title-dash-number", "Catch-22 - 001.mkv", sdk.ParsedMedia{Title: "Catch-22 - 001", Class: "movie"}},
		{"numbered-show-title-dash-number", "The 4400 - 001.mkv", sdk.ParsedMedia{Title: "The 4400 - 001", Class: "movie"}},
		{"film-library-dash-number", "/media/Movies/Bond - 007.mkv", sdk.ParsedMedia{Title: "Bond - 007", Class: "movie"}},
		// An unpadded number after " - " needs the same context, and a single
		// digit is a sequel number even in a folder named for the film.
		{"dash-two-digits-in-film-name", "Movie - 12.mkv", sdk.ParsedMedia{Title: "Movie - 12", Class: "movie"}},
		{"film-library-dash-two-digits", "/media/Movies/Saw - 10.mkv", sdk.ParsedMedia{Title: "Saw - 10", Class: "movie"}},
		{"sequel-digit-in-its-folder", "/Videos/Rocky/Rocky - 2.mkv", sdk.ParsedMedia{Title: "Rocky - 2", Class: "movie"}},
		{"sequel-digit-under-tv", "/media/TV/Show - 2.mkv", sdk.ParsedMedia{Title: "Show - 2"}},
		{"trailing-number-in-film-title", "/media/Movies/Ocean's 11.mkv", sdk.ParsedMedia{Title: "Ocean's 11", Class: "movie"}},
		{"trailing-number-with-year", "/media/Movies/Apollo 13 (1995).mkv", sdk.ParsedMedia{Title: "Apollo 13", Class: "movie", Year: 1995}},
		{"resolution-after-dash-is-not-absolute", "/media/TV/Show - 1080p.mkv", sdk.ParsedMedia{Title: "Show"}},
		// A camera clip stem is never a series, even under a TV folder.
		{"camera-clip", "VID_0001.mp4", sdk.ParsedMedia{Title: "VID 0001", Class: "movie"}},
		{"camera-still-clip", "DSC_0012.mov", sdk.ParsedMedia{Title: "DSC 0012", Class: "movie"}},
		{"camera-clip-under-tv", "/media/TV/PXL_0003.mp4", sdk.ParsedMedia{Title: "PXL 0003"}},
		// A bare-e code needs episode context, and a channel layout's ".1e01"
		// ("DD5.1e01") is never a code.
		{"bare-e-without-context", "Movie 2e10.mkv", sdk.ParsedMedia{Title: "Movie 2e10", Class: "movie"}},
		{"bare-e-after-channel-layout", "Movie.1080p.DD5.1e01.mkv", sdk.ParsedMedia{Title: "Movie", Class: "movie"}},
		{"bare-e-after-channel-layout-under-tv", "/media/TV/Movie.1080p.DD5.1e01.mkv", sdk.ParsedMedia{Title: "Movie"}},
		// A show folder of the same name is no context for an underscored
		// counter: a home video folder holds its clips the same way.
		{"home-video-in-its-folder", "/Home/Wedding/Wedding_0002.mp4", sdk.ParsedMedia{Title: "Wedding 0002", Class: "movie"}},
		{"drone-clip-in-its-folder", "/Videos/DJI/DJI_0001.MP4", sdk.ParsedMedia{Title: "DJI 0001", Class: "movie"}},
		{"samsung-clip", "/Videos/SAM_0001.mp4", sdk.ParsedMedia{Title: "SAM 0001", Class: "movie"}},
		{"casio-clip-under-tv", "/media/TV/CIMG_0001.avi", sdk.ParsedMedia{Title: "CIMG 0001"}},
		{"pentax-clip-under-tv", "/media/TV/IMGP_0001.avi", sdk.ParsedMedia{Title: "IMGP 0001"}},
		// The nearest library folder decides: a film folder below a TV one
		// holds films.
		{"film-folder-below-tv", "/Media/TV/Movies/Bond - 007.mkv", sdk.ParsedMedia{Title: "Bond - 007", Class: "movie"}},
		// A release bracket alone is no context, and never in a film folder.
		{"release-bracket-in-film-folder", "/Movies/[YTS] Agent 0011.mkv", sdk.ParsedMedia{Title: "[YTS] Agent 0011", Class: "movie"}},
		{"release-bracket-alone", "/Downloads/[YTS] Agent 0011.mkv", sdk.ParsedMedia{Title: "[YTS] Agent 0011", Class: "movie"}},
		{"release-bracket-checksum-in-film-folder", "/Movies/[Grp] Show - 001 [ABCD1234].mkv", sdk.ParsedMedia{Title: "[Grp] Show - 001", Class: "movie"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertParse(t, parser, tc.path, tc.want)
		})
	}
}

// TestEpisodeFormsReadWithContext pins the weak TV name forms where the path
// says the file is an episode: a season folder, a show folder of the series'
// name, a TV or anime folder, or an anime group or checksum bracket.
func TestEpisodeFormsReadWithContext(t *testing.T) {
	parser := filmtv.New()
	cases := []struct {
		name, path string
		want       sdk.ParsedMedia
	}{
		{"absolute-in-show-folder", "/media/Dealbreakers Talk Show/Dealbreakers Talk Show 0001.mkv",
			sdk.ParsedMedia{Title: "Dealbreakers Talk Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		{"absolute-under-tv-folder", "/media/TV/Show 0001 1080p.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		{"absolute-in-season-folder", "/media/Show/Season 2/Show 0012.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 12, AbsoluteOrdinal: 12}},
		{"anime-dash-number-with-title", "/media/Show/Show - 001 - Title.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		{"four-digits-with-title", "/media/Show/Show 0001 - Title.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		{"anime-group-and-checksum", "[Grp] Show - 001 [ABCD1234].mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		// The checksum counts although the title stops before it.
		{"anime-checksum-only", "/Downloads/Show - 001 [ABCD1234].mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		{"anime-group-and-dash-number", "/Downloads/[Grp] Show - 001.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Index: 1, AbsoluteOrdinal: 1}},
		// The dashed form needs no zero padding: two to four digits, with an
		// optional release revision (CORE-2700).
		{"anime-unpadded-two-digits-in-season-folder", "/tv/Mob Psycho 100/Season 01/Mob Psycho 100 - 09 [1080p].mkv",
			sdk.ParsedMedia{Title: "Mob Psycho 100", Class: "episode", Index: 9, AbsoluteOrdinal: 9}},
		{"anime-unpadded-three-digits-with-title", "/tv/Naruto Shippuden/Naruto Shippuden - 107 - Strange Bedfellows.mkv",
			sdk.ParsedMedia{Title: "Naruto Shippuden", Class: "episode", Index: 107, AbsoluteOrdinal: 107}},
		{"anime-unpadded-four-digits-in-show-folder", "/tv/One Piece/One Piece - 1071.mkv",
			sdk.ParsedMedia{Title: "One Piece", Class: "episode", Index: 1071, AbsoluteOrdinal: 1071}},
		{"anime-unpadded-with-group-under-anime", "/anime/Kingdom/[Erai-raws] Kingdom 3rd Season - 02 [1080p].mkv",
			sdk.ParsedMedia{Title: "Kingdom 3rd Season", Class: "episode", Index: 2, AbsoluteOrdinal: 2}},
		{"anime-unpadded-with-revision", "/Downloads/[SubsPlease] Frieren - 12v2 (1080p) [ABCD1234].mkv",
			sdk.ParsedMedia{Title: "Frieren", Class: "episode", Index: 12, AbsoluteOrdinal: 12}},
		{"bare-e-in-season-folder", "/media/Show/Season 1/1e01-Pilot.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1}},
		{"bare-e-under-tv-folder", "/media/TV/Show 1e01.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1}},
		// The code carries its own season: a season folder naming another one
		// leaves the code's season (matching then decides), not a film.
		{"bare-e-disagrees-with-folder", "/TV/Show/Season 2/Show 1e01.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 1}},
		{"bare-e-alone-disagrees-with-folder", "/media/TV/Show/Season 2/1e10.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "1", Index: 10}},
		// "Specials" is season 0: a leading pair or bare-e code there reads as
		// S00, whatever season number the name gives.
		{"leading-zero-pair-in-specials", "/media/TV/Synthetic Show/Specials/0 02 Behind the Scenes.mkv",
			sdk.ParsedMedia{Title: "Synthetic Show", Class: "episode", Sequence: "0", Index: 2}},
		{"leading-pair-in-specials", "/media/Show/Specials/1 01 X.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "0", Index: 1}},
		{"bare-e-in-specials", "/media/Show/Specials/1e01 X.mkv",
			sdk.ParsedMedia{Title: "Show", Class: "episode", Sequence: "0", Index: 1}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			assertParse(t, parser, tc.path, tc.want)
		})
	}
	if got := parser.MediaTitle("/media/Show/Show - 001 - Title.mkv", sdk.ParseProbe{}); got != "E01 · Title" {
		t.Errorf("MediaTitle of an absolute episode with a title = %q, want %q", got, "E01 · Title")
	}
	if got := parser.MediaTitle("/tv/Naruto Shippuden/Naruto Shippuden - 107 - Strange Bedfellows.mkv", sdk.ParseProbe{}); got != "E107 · Strange Bedfellows" {
		t.Errorf("MediaTitle of an unpadded absolute episode with a title = %q, want %q", got, "E107 · Strange Bedfellows")
	}
}

// TestArtworkIsNotMedia pins that the artwork a library keeps beside its media
// never parses as a film.
func TestArtworkIsNotMedia(t *testing.T) {
	parser := filmtv.New()
	for _, path := range []string{"/media/TV/Show/season01-banner.jpg", "/media/Movies/Film (2001)/poster.PNG", "/media/TV/Show/Season 1/folder.jpeg"} {
		if got, ok := parser.Parse(path, sdk.ParseProbe{}); ok {
			t.Errorf("Parse(%q) = %+v, want refused", path, got)
		}
	}
}

func assertParse(t *testing.T, parser *filmtv.Parser, path string, want sdk.ParsedMedia) {
	t.Helper()
	got, ok := parser.Parse(path, sdk.ParseProbe{})
	if !ok {
		t.Fatalf("Parse(%q) refused, want %+v", path, want)
	}
	if got.Title != want.Title || got.Class != want.Class || got.Year != want.Year ||
		got.Sequence != want.Sequence || got.Index != want.Index || got.AbsoluteOrdinal != want.AbsoluteOrdinal {
		t.Errorf("Parse(%q) = %+v, want %+v", path, got, want)
	}
}
