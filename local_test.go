package filmtv_test

import (
	"testing"

	"github.com/beamfall/plugin-filmtv"
	sdk "github.com/beamfall/plugin-sdk"
)

func TestLocalHydration(t *testing.T) {
	cases := []struct {
		path string
		want filmtv.LocalGrouping
	}{
		{
			path: "/media/TV/Pioneer One/Season 01/Pioneer One - S01E02 - The Man From Orbit.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Pioneer One", SeasonNumber: 1, SeasonTitle: "Season 1", EpisodeIndex: 2, EpisodeTitle: "The Man From Orbit"},
		},
		{
			path: "/media/TV/Pioneer One/Specials/Pioneer.One.S00E01.1080p.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Pioneer One", SeasonNumber: 0, SeasonTitle: "Specials", EpisodeIndex: 1},
		},
		{
			path: "/media/TV/Pioneer One/Season 02/Pioneer.One.S02E03-E04.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Pioneer One", SeasonNumber: 2, SeasonTitle: "Season 2", EpisodeIndex: 3, EpisodeIndexEnd: 4},
		},
		{
			path: "/media/TV/The IT Crowd/Season 1/1e01-Yesterday's Jam.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "The IT Crowd", SeasonNumber: 1, SeasonTitle: "Season 1", EpisodeIndex: 1, EpisodeTitle: "Yesterday's Jam"},
		},
		{
			path: "/media/TV/Synthetic Show/Season 3/3 01 From Hell.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Synthetic Show", SeasonNumber: 3, SeasonTitle: "Season 3", EpisodeIndex: 1, EpisodeTitle: "From Hell"},
		},
		{
			// An absolute-numbered episode groups under season 1.
			path: "/media/TV/Dealbreakers Talk Show/Dealbreakers Talk Show 0001.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Dealbreakers Talk Show", SeasonNumber: 1, SeasonTitle: "Season 1", EpisodeIndex: 1},
		},
		{
			// Inside a season folder it groups under that season.
			path: "/media/TV/Show/Season 2/Show - 001.mkv",
			want: filmtv.LocalGrouping{Episode: true, SeriesTitle: "Show", SeasonNumber: 2, SeasonTitle: "Season 2", EpisodeIndex: 1},
		},
		{
			path: "/media/Movies/Sintel (2010)/Sintel (2010).mkv",
			want: filmtv.LocalGrouping{},
		},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := filmtv.LocalHydration(tc.path); got != tc.want {
				t.Errorf("LocalHydration(%q) = %+v, want %+v", tc.path, got, tc.want)
			}
		})
	}
}

// TestMediaTitleKeepsParsedSeriesTitle pins the split between the display
// title and the parsed title: the parsed title stays the series name (the
// provider search query) while MediaTitle carries the episode's code and its
// own title.
func TestMediaTitleKeepsParsedSeriesTitle(t *testing.T) {
	p := &filmtv.Parser{}
	cases := []struct {
		path, wantTitle, wantParsed string
	}{
		{"/media/TV/Pioneer One/Season 01/Pioneer One - S01E01 - Earthfall.mkv", "S01E01 · Earthfall", "Pioneer One"},
		{"/media/TV/Pioneer One/Season 01/Pioneer.One.S01E01.1080p.mkv", "S01E01", "Pioneer One"},
		// Name forms seen on a real library (CORE-2313): the episode keeps
		// its code and its own title, apostrophe included, never the raw
		// filename.
		{"/media/TV/The IT Crowd/Season 1/1e01-Yesterday's Jam.mkv", "S01E01 · Yesterday's Jam", "The IT Crowd"},
		{"/media/TV/Synthetic Show/Season 3/3 01 From Hell.mkv", "S03E01 · From Hell", "Synthetic Show"},
		{"/media/TV/Dealbreakers Talk Show/Dealbreakers Talk Show 0001.mkv", "E01", "Dealbreakers Talk Show"},
		// A release tag is never the episode's title (CORE-2470).
		{"/mnt/user/tv/Ted Lasso/Season 3/ted.lasso.s03e08.dv.2160p.web.h265-ggez.mkv", "S03E08", "ted lasso"},
		{"/media/Movies/Sintel (2010)/Sintel (2010).mkv", "", "Sintel"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			if got := p.MediaTitle(tc.path, sdk.ParseProbe{}); got != tc.wantTitle {
				t.Errorf("MediaTitle(%q) = %q, want %q", tc.path, got, tc.wantTitle)
			}
			parsed, ok := p.Parse(tc.path, sdk.ParseProbe{})
			if !ok || parsed.Title != tc.wantParsed {
				t.Errorf("Parse(%q).Title = %q (ok=%v), want %q", tc.path, parsed.Title, ok, tc.wantParsed)
			}
		})
	}
}

// TestIdentifiedMediaTitleUsesTheIdentifiedEpisodeName pins CORE-2470: when
// an identification carries the episode's own name, the display title uses it,
// punctuation included, in place of the filename's title, a release tag or a
// bare code; without one it is MediaTitle.
func TestIdentifiedMediaTitleUsesTheIdentifiedEpisodeName(t *testing.T) {
	p := &filmtv.Parser{}
	const dir = "/mnt/user/tv/Ted Lasso/Season 3/"
	cases := []struct {
		name, path, identified, want string
	}{
		{"replaces-filename-title", dir + "Ted.Lasso.S03E02.I.Dont.Want.to.Go.to.Chelsea.2160p.ATVP.WEB-DL.DDP5.1.HDR.H.265-NTb.mkv",
			"I Don't Want to Go to Chelsea", "S03E02 · I Don't Want to Go to Chelsea"},
		{"names-a-bare-code", dir + "Ted.Lasso.S03E01.2160p.WEB.H265-GGEZ.mkv",
			"Smells Like Mean Spirit", "S03E01 · Smells Like Mean Spirit"},
		{"replaces-a-release-tag", dir + "ted.lasso.s03e08.dv.2160p.web.h265-ggez.mkv",
			"We'll Never Have Paris", "S03E08 · We'll Never Have Paris"},
		{"keeps-a-tag-like-real-name", dir + "Ted.Lasso.S03E05.2160p.WEB.H265-GGEZ.mkv",
			"HDR", "S03E05 · HDR"},
		{"whitespace-collapsed", dir + "Ted.Lasso.S03E01.2160p.WEB.H265-GGEZ.mkv",
			"  Smells  Like Mean Spirit ", "S03E01 · Smells Like Mean Spirit"},
		{"none-keeps-filename-title", dir + "Ted.Lasso.S03E06.Sunflowers.HDR.2160p.WEB.H265-GGEZ.mkv",
			"", "S03E06 · Sunflowers"},
		{"none-keeps-bare-code", dir + "ted.lasso.s03e09.hdr.2160p.web.h265-ggez.mkv",
			" ", "S03E09"},
		{"not-an-episode", "/media/Movies/Sintel (2010)/Sintel (2010).mkv", "Sintel", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			fields := map[string]string{filmtv.EpisodeTitleField: tc.identified, "network": "Apple TV+"}
			if got := p.IdentifiedMediaTitle(tc.path, sdk.ParseProbe{}, fields); got != tc.want {
				t.Errorf("IdentifiedMediaTitle(%q, %q) = %q, want %q", tc.path, tc.identified, got, tc.want)
			}
		})
	}
	if got, want := p.IdentifiedMediaTitle(dir+"Ted.Lasso.S03E06.Sunflowers.HDR.2160p.WEB.H265-GGEZ.mkv", sdk.ParseProbe{}, nil), "S03E06 · Sunflowers"; got != want {
		t.Errorf("IdentifiedMediaTitle with no fields = %q, want %q", got, want)
	}
}

func TestHydratedFieldDeclared(t *testing.T) {
	for _, field := range filmtv.Bundle().MediaFields {
		if field.Name == filmtv.HydratedField {
			if field.Type != "flag" {
				t.Fatalf("%s is declared as %+v, want a flag", filmtv.HydratedField, field)
			}
			return
		}
	}
	t.Fatalf("bundle declares no %s Media field", filmtv.HydratedField)
}

// TestSeasonRelationsDeclared pins the relations Core's hydration seam writes
// for a local episode: a season holds its episodes and belongs to a show.
func TestSeasonRelationsDeclared(t *testing.T) {
	want := map[string]string{
		filmtv.SeasonEpisodeRelation: sdk.MediaKindName,
		filmtv.SeasonOfRelation:      "show",
	}
	for _, kind := range filmtv.Bundle().EntityKinds {
		if kind.Name != "season" {
			continue
		}
		for _, rel := range kind.Relations {
			to, ok := want[rel.Name]
			if !ok {
				continue
			}
			if rel.ToKind != to {
				t.Errorf("season relation %q = %+v, want ToKind %q", rel.Name, rel, to)
			}
			// Core's store enforces "one" on Media-targeted relations only: an
			// episode belongs to a single season.
			if rel.Name == filmtv.SeasonEpisodeRelation && rel.Cardinality != "one" {
				t.Errorf("season relation %q cardinality = %q, want one", rel.Name, rel.Cardinality)
			}
			delete(want, rel.Name)
		}
	}
	for name := range want {
		t.Errorf("season Kind declares no %q relation", name)
	}
}

func TestHomeListsShows(t *testing.T) {
	for _, layout := range filmtv.Bundle().Layouts {
		for _, section := range layout.Sections {
			if section.Source.Kind == "entity-items" && section.Source.EntityKind == "show" {
				return
			}
		}
	}
	t.Fatal("no layout section lists show Entities")
}
