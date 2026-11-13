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
// provider search query) while MediaTitle carries the episode's own title.
func TestMediaTitleKeepsParsedSeriesTitle(t *testing.T) {
	p := &filmtv.Parser{}
	cases := []struct {
		path, wantTitle, wantParsed string
	}{
		{"/media/TV/Pioneer One/Season 01/Pioneer One - S01E01 - Earthfall.mkv", "Earthfall", "Pioneer One"},
		{"/media/TV/Pioneer One/Season 01/Pioneer.One.S01E01.1080p.mkv", "", "Pioneer One"},
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
