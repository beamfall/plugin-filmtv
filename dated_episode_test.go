package filmtv_test

import (
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	filmtv "github.com/beamfall/plugin-filmtv"
	sdk "github.com/beamfall/plugin-sdk"
)

// syntheticTVCorpus is the CORE-1713 reproduction: two days of one daily show
// and a multi-episode file with no episode title.
var syntheticTVCorpus = []string{
	"/media/TV/Synthetic Daily/Synthetic Daily 2023-03-02.mkv",
	"/media/TV/Synthetic Daily/Synthetic Daily 2023-03-01.mkv",
	"/media/TV/Synthetic Show/Season 01/Synthetic Show S01E03-E04.mkv",
}

func TestDatedEpisodeParse(t *testing.T) {
	p := filmtv.New()
	cases := []struct {
		path         string
		want         sdk.ParsedMedia
		airDate      string
		episodeTitle string
	}{
		{path: "Synthetic Daily 2023-03-01.mkv",
			want: sdk.ParsedMedia{Title: "Synthetic Daily", Class: "episode"}, airDate: "2023-03-01"},
		{path: "Synthetic.Daily.2023.03.02.Guest.Night.720p.mkv",
			want:    sdk.ParsedMedia{Title: "Synthetic Daily", Class: "episode"},
			airDate: "2023-03-02", episodeTitle: "Guest Night"},
		{path: "Synthetic_Daily_2024_02_29.mkv",
			want: sdk.ParsedMedia{Title: "Synthetic Daily", Class: "episode"}, airDate: "2024-02-29"},
		{path: "Synthetic Daily (2019) 2023 03 05 - Finale.mkv",
			want:    sdk.ParsedMedia{Title: "Synthetic Daily", Class: "episode", Year: 2019},
			airDate: "2023-03-05", episodeTitle: "Finale"},
		// Not calendar dates: read as before (a movie with a year).
		{path: "Synthetic Daily 2023-13-01.mkv", want: sdk.ParsedMedia{Title: "Synthetic Daily", Class: "movie", Year: 2023}},
		{path: "Synthetic Daily 2023-02-30.mkv", want: sdk.ParsedMedia{Title: "Synthetic Daily", Class: "movie", Year: 2023}},
		// A season/episode token wins over a date.
		{path: "Synthetic Show S01E02 2023-03-01.mkv",
			want:         sdk.ParsedMedia{Title: "Synthetic Show", Class: "episode", Sequence: "1", Index: 2},
			episodeTitle: "2023-03-01"},
	}
	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			got, ok := p.Parse(tc.path, sdk.ParseProbe{})
			if !ok || !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("Parse(%q) = %+v (ok=%v), want %+v", tc.path, got, ok, tc.want)
			}
			var want []sdk.ParsedField
			if tc.episodeTitle != "" {
				want = append(want, sdk.ParsedField{Name: filmtv.EpisodeTitleField, Text: tc.episodeTitle})
			}
			if tc.airDate != "" {
				date, err := time.Parse(time.DateOnly, tc.airDate)
				if err != nil {
					t.Fatal(err)
				}
				want = append(want, sdk.ParsedField{Name: filmtv.AirDateField, Date: date})
			}
			if fields := p.ParseTypedMediaFields(tc.path, sdk.ParseProbe{}); !reflect.DeepEqual(fields, want) {
				t.Errorf("ParseTypedMediaFields(%q) = %+v, want %+v", tc.path, fields, want)
			}
		})
	}
}

// TestSyntheticCorpusDisplayTitles is the ticket's acceptance: both daily files
// get distinct air-date titles that sort by date, the multi-episode file shows
// its range, and every episode's card title carries SxxEyy or the date.
func TestSyntheticCorpusDisplayTitles(t *testing.T) {
	p := filmtv.New()
	titles := make([]string, 0, len(syntheticTVCorpus))
	for _, path := range syntheticTVCorpus {
		titles = append(titles, p.MediaTitle(path, sdk.ParseProbe{}))
	}
	want := []string{"2023-03-02", "2023-03-01", "S01E03-04"}
	if !slices.Equal(titles, want) {
		t.Fatalf("display titles = %q, want %q", titles, want)
	}
	daily := slices.Clone(titles[:2])
	slices.Sort(daily)
	if !slices.Equal(daily, []string{"2023-03-01", "2023-03-02"}) {
		t.Errorf("daily titles sort as %q, want air-date order", daily)
	}
}

func TestMediaTitleEpisodeCode(t *testing.T) {
	p := filmtv.New()
	cases := []struct{ path, want string }{
		{"Synthetic Show - S01E01 - Opening Night.mkv", "S01E01 · Opening Night"},
		{"Synthetic.Show.S01E01.1080p.mkv", "S01E01"},
		{"Synthetic.Show.S02E09E10.mkv", "S02E09-10"},
		{"Synthetic Show - 1x01-1x02 - Pilot.mkv", "S01E01-02 · Pilot"},
		{"Synthetic Show S00E01.mkv", "S00E01"},
		{"Synthetic Show S01E01-S02E03.mkv", "S01E01"},
		{"Synthetic.Daily.2023.03.02.Guest.Night.720p.mkv", "2023-03-02 · Guest Night"},
		{"Synthetic Film (2010).mkv", ""},
	}
	for _, tc := range cases {
		if got := p.MediaTitle(tc.path, sdk.ParseProbe{}); got != tc.want {
			t.Errorf("MediaTitle(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

// TestEpisodeDisplayTitlesSortInEpisodeOrder pins that ordering one series'
// display titles (the landing grid's title sort) orders its episodes.
func TestEpisodeDisplayTitlesSortInEpisodeOrder(t *testing.T) {
	p := filmtv.New()
	paths := []string{
		"Synthetic Show S02E01 - Return.mkv",
		"Synthetic Show S01E10 - Late.mkv",
		"Synthetic Show S01E02 - Early.mkv",
		"Synthetic Show S01E03-E04 - Double.mkv",
	}
	var titles []string
	for _, path := range paths {
		titles = append(titles, p.MediaTitle(path, sdk.ParseProbe{}))
	}
	slices.Sort(titles)
	want := []string{"S01E02 · Early", "S01E03-04 · Double", "S01E10 · Late", "S02E01 · Return"}
	if !slices.Equal(titles, want) {
		t.Errorf("sorted titles = %q, want %q", titles, want)
	}
	for _, title := range titles {
		if !strings.HasPrefix(title, "S") {
			t.Errorf("title %q carries no SxxEyy code", title)
		}
	}
}

func TestLocalHydrationDatedEpisode(t *testing.T) {
	got := filmtv.LocalHydration("/media/TV/Synthetic Daily/Synthetic Daily 2023-03-01 - Opening.mkv")
	want := filmtv.LocalGrouping{
		Episode:      true,
		SeriesTitle:  "Synthetic Daily",
		SeasonNumber: 2023,
		SeasonTitle:  "Season 2023",
		EpisodeIndex: 301,
		EpisodeTitle: "Opening",
		AirDate:      "2023-03-01",
	}
	if got != want {
		t.Errorf("LocalHydration = %+v, want %+v", got, want)
	}
}

func TestAirDateFieldDeclaredAsDate(t *testing.T) {
	for _, field := range filmtv.Bundle().MediaFields {
		if field.Name == filmtv.AirDateField {
			if field.Type != "date" || !field.Sortable {
				t.Fatalf("%s is declared as %+v, want a sortable date", filmtv.AirDateField, field)
			}
			return
		}
	}
	t.Fatalf("bundle declares no %s Media field", filmtv.AirDateField)
}
