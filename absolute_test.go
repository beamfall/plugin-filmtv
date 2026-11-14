package filmtv_test

import (
	"os"
	"testing"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// The fixture keeps the shape of a TVmaze /shows/{id}?embed=episodes response
// with synthetic values: 12 regular season-1 episodes, a special with a null
// number, then 12 regular season-2 episodes.
func loadTVmazeEpisodes(t *testing.T) []filmtv.EpisodeRef {
	t.Helper()
	data, err := os.ReadFile("testdata/tvmaze-show-episodes.json")
	if err != nil {
		t.Fatal(err)
	}
	episodes, err := filmtv.DecodeTVmazeEpisodes(data)
	if err != nil {
		t.Fatal(err)
	}
	return episodes
}

func TestDecodeTVmazeEpisodesDropsSpecials(t *testing.T) {
	episodes := loadTVmazeEpisodes(t)
	if len(episodes) != 24 {
		t.Fatalf("decoded %d regular episodes, want 24", len(episodes))
	}
	for _, episode := range episodes {
		if episode.Season < 1 || episode.Number < 1 {
			t.Errorf("decoded a non-regular episode %+v", episode)
		}
	}
}

func TestDecodeTVmazeEpisodesRejectsMalformedDocument(t *testing.T) {
	if _, err := filmtv.DecodeTVmazeEpisodes([]byte(`{"_embedded": [`)); err == nil {
		t.Fatal("malformed document decoded without error")
	}
}

func TestAbsoluteEpisodeOverTVmazeList(t *testing.T) {
	episodes := loadTVmazeEpisodes(t)
	cases := []struct {
		absolute int
		want     filmtv.EpisodeRef
		ok       bool
	}{
		{absolute: 1, want: filmtv.EpisodeRef{Season: 1, Number: 1}, ok: true},
		{absolute: 12, want: filmtv.EpisodeRef{Season: 1, Number: 12}, ok: true},
		// The special between the seasons is not counted.
		{absolute: 13, want: filmtv.EpisodeRef{Season: 2, Number: 1}, ok: true},
		{absolute: 24, want: filmtv.EpisodeRef{Season: 2, Number: 12}, ok: true},
		{absolute: 25},
		{absolute: 0},
		{absolute: -3},
	}
	for _, tc := range cases {
		got, ok := filmtv.AbsoluteEpisode(episodes, tc.absolute)
		if ok != tc.ok || got != tc.want {
			t.Errorf("AbsoluteEpisode(%d) = %+v, %v; want %+v, %v", tc.absolute, got, ok, tc.want, tc.ok)
		}
	}
}

func TestAbsoluteEpisodeOrdersAndSkipsSeasonZero(t *testing.T) {
	// Out of order, with a season-0 entry and a zero-numbered entry.
	episodes := []filmtv.EpisodeRef{
		{Season: 2, Number: 2}, {Season: 0, Number: 1}, {Season: 1, Number: 2},
		{Season: 2, Number: 1}, {Season: 1, Number: 1}, {Season: 1, Number: 0},
	}
	want := []filmtv.EpisodeRef{{Season: 1, Number: 1}, {Season: 1, Number: 2}, {Season: 2, Number: 1}, {Season: 2, Number: 2}}
	for i, w := range want {
		got, ok := filmtv.AbsoluteEpisode(episodes, i+1)
		if !ok || got != w {
			t.Errorf("AbsoluteEpisode(%d) = %+v, %v; want %+v", i+1, got, ok, w)
		}
	}
	if _, ok := filmtv.AbsoluteEpisode(episodes, len(want)+1); ok {
		t.Error("mapped past the last regular episode")
	}
	if _, ok := filmtv.AbsoluteEpisode(nil, 1); ok {
		t.Error("mapped over an empty list")
	}
}
