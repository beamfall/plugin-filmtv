package filmtv_test

import (
	"reflect"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestYearPickerKeepsYearNamedTitles pins the year picker for names whose title
// is or holds a year (CORE-2701): a standalone year with no title text before
// it, or directly followed by another year, stays in the title; a leading
// parenthesized year is the year; title-then-year names are unchanged.
func TestYearPickerKeepsYearNamedTitles(t *testing.T) {
	cases := []struct {
		path string
		want sdk.ParsedMedia
		ok   bool
	}{
		{"/movies/2012.2009.1080p.BluRay.x264.mkv", sdk.ParsedMedia{Title: "2012", Class: "movie", Year: 2009}, true},
		{"/movies/2019 After The Fall Of New York 1983 REMASTERED BDRip x264-GHOULS.mkv", sdk.ParsedMedia{Title: "2019 After The Fall Of New York", Class: "movie", Year: 1983}, true},
		{"/movies/(2000) The Emperor's New Groove.avi", sdk.ParsedMedia{Title: "The Emperor's New Groove", Class: "movie", Year: 2000}, true},
		{"/movies/(2000) The Emperor's New Groove 1080p BluRay.mkv", sdk.ParsedMedia{Title: "The Emperor's New Groove", Class: "movie", Year: 2000}, true},
		{"/movies/[Group] (2000) The Emperor's New Groove.mkv", sdk.ParsedMedia{Title: "The Emperor's New Groove", Class: "movie", Year: 2000}, true},
		{"/movies/(2009) 2012.mkv", sdk.ParsedMedia{Title: "2012", Class: "movie", Year: 2009}, true},
		{"/movies/2001.A.Space.Odyssey.1968.1080p.mkv", sdk.ParsedMedia{Title: "2001 A Space Odyssey", Class: "movie", Year: 1968}, true},
		{"/movies/1917.2019.1080p.BluRay.mkv", sdk.ParsedMedia{Title: "1917", Class: "movie", Year: 2019}, true},
		{"/movies/1984.1984.mkv", sdk.ParsedMedia{Title: "1984", Class: "movie", Year: 1984}, true},
		{"/movies/Blade.Runner.2049.2017.1080p.mkv", sdk.ParsedMedia{Title: "Blade Runner 2049", Class: "movie", Year: 2017}, true},
		{"/movies/Blade Runner 2049 - 2017.mkv", sdk.ParsedMedia{Title: "Blade Runner 2049", Class: "movie", Year: 2017}, true},
		{"/movies/2012.mkv", sdk.ParsedMedia{Title: "2012", Class: "movie"}, true},
		{"/movies/2001.A.Space.Odyssey.1080p.mkv", sdk.ParsedMedia{Title: "2001 A Space Odyssey", Class: "movie"}, true},
		{"/tv/1923/Season 01/1923.S01E01.mkv", sdk.ParsedMedia{Title: "1923", Class: "episode", Sequence: "1", Index: 1}, true},
		{"/movies/(2000).avi", sdk.ParsedMedia{}, false},
		// A leading year must leave a title: otherwise the text before it is the title.
		{"/movies/[REC] (2007).mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/movies/[REC] (2007) 1080p BluRay.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/movies/$ (1971).mkv", sdk.ParsedMedia{Title: "$", Class: "movie", Year: 1971}, true},
		{"/movies/[REC].2007.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/movies/[REC] (2007) PROPER.1080p.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/movies/[Group] (2013) DV [DEADBEEF].mkv", sdk.ParsedMedia{Title: "DV", Class: "movie", Year: 2013}, true},
		{"/incoming/[Group] (2018) ReMastered S01E01.mkv", sdk.ParsedMedia{Title: "ReMastered", Class: "episode", Year: 2018, Sequence: "1", Index: 1}, true},
		{"/movies/[REC] 2007-2008-2009.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/tv/[REC] (2007) PROPER.1080p/Season 01/S01E01.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "episode", Year: 2007, Sequence: "1", Index: 1}, true},
		{"/movies/– (2000) The Emperor.mkv", sdk.ParsedMedia{Title: "The Emperor", Class: "movie", Year: 2000}, true},
		{"/movies/[Group (1999)] (2000) The Emperor's New Groove.avi", sdk.ParsedMedia{Title: "The Emperor's New Groove", Class: "movie", Year: 2000}, true},
		// A tight rising range is a run of years.
		{"/movies/[REC] 2007-2009.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "movie", Year: 2007}, true},
		{"/movies/Fringe 2008-2013.mkv", sdk.ParsedMedia{Title: "Fringe", Class: "movie", Year: 2008}, true},
		// Episodes: the show folder settles a bare-year series name.
		{"/tv/Doctor Who (2005)/Season 01/2005.S01E01.mkv", sdk.ParsedMedia{Title: "Doctor Who", Class: "episode", Year: 2005, Sequence: "1", Index: 1}, true},
		{"/incoming/Fringe 2008-2013/Fringe 0001.mkv", sdk.ParsedMedia{Title: "Fringe", Class: "episode", Index: 1, AbsoluteOrdinal: 1}, true},
		{"/tv/Show/Season 01/(2000) Show 0001.mkv", sdk.ParsedMedia{Title: "Show", Class: "episode", Year: 2000, Index: 1, AbsoluteOrdinal: 1}, true},
		{"/incoming/[Group] 1923.S01E01.mkv", sdk.ParsedMedia{Title: "1923", Class: "episode", Sequence: "1", Index: 1}, true},
		{"/tv/[REC] (2007)/Season 01/S01E01.mkv", sdk.ParsedMedia{Title: "[REC]", Class: "episode", Year: 2007, Sequence: "1", Index: 1}, true},
		{"/tv/Space 1999/Season 01/Space_1999_0001.mkv", sdk.ParsedMedia{Title: "Space 1999", Class: "episode", Index: 1, AbsoluteOrdinal: 1}, true},
		{"/tv/1923 - 12.mkv", sdk.ParsedMedia{Title: "1923", Class: "episode", Index: 12, AbsoluteOrdinal: 12}, true},
		// Controls: a title before the year keeps the old reading.
		{"/movies/1917 (2019).mkv", sdk.ParsedMedia{Title: "1917", Class: "movie", Year: 2019}, true},
		{"/movies/2001 A Space Odyssey (1968).mkv", sdk.ParsedMedia{Title: "2001 A Space Odyssey", Class: "movie", Year: 1968}, true},
		{"/movies/Blade Runner 2049 (2017).mkv", sdk.ParsedMedia{Title: "Blade Runner 2049", Class: "movie", Year: 2017}, true},
		{"/movies/2012 (2009) - 1080p.mkv", sdk.ParsedMedia{Title: "2012", Class: "movie", Year: 2009, VersionLabel: "1080p"}, true},
		{"/movies/Heat.1995.1080p.mkv", sdk.ParsedMedia{Title: "Heat", Class: "movie", Year: 1995}, true},
	}
	parser := filmtv.New()
	for _, c := range cases {
		got, ok := parser.Parse(c.path, sdk.ParseProbe{})
		if ok != c.ok || ok && !reflect.DeepEqual(got, c.want) {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", c.path, got, ok, c.want, c.ok)
		}
	}
}
