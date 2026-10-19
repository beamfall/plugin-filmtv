package mediaimport

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseJellyfinWatchStates(t *testing.T) {
	input := strings.NewReader(`[
  {
    "ProviderIds": {"Tmdb": "12345", "Imdb": "tt1234567"},
    "UserData": {
      "Played": true,
      "PlaybackPositionTicks": 920000000,
      "LastPlayedDate": "2026-06-10T12:34:56Z"
    }
  },
  {
    "ProviderIds": {"Tvdb": "999"},
    "UserData": {"Played": false, "PlaybackPositionTicks": 0}
  }
]`)

	got, err := ParseJellyfinWatchStates(input)
	if err != nil {
		t.Fatalf("ParseJellyfinWatchStates() error = %v", err)
	}

	wantTime := time.Date(2026, 6, 10, 12, 34, 56, 0, time.UTC)
	want := []WatchState{
		{ExternalID: ExternalID{Source: "tmdb", ID: "12345"}, Watched: true, PositionSeconds: 92, LastPlayed: wantTime},
		{ExternalID: ExternalID{Source: "imdb", ID: "tt1234567"}, Watched: true, PositionSeconds: 92, LastPlayed: wantTime},
		{ExternalID: ExternalID{Source: "tvdb", ID: "999"}, Watched: false},
	}
	assertWatchStates(t, got, want)
}

func TestParseEmbyWatchStates(t *testing.T) {
	input := strings.NewReader(`{
  "Items": [
    {
      "ProviderIds": {"Tmdb": "24680"},
      "UserData": {
        "Played": true,
        "PlaybackPositionTicks": 1200000000,
        "LastPlayedDate": "2026-06-11T01:02:03.0000000Z"
      }
    }
  ]
}`)

	got, err := ParseEmbyWatchStates(input)
	if err != nil {
		t.Fatalf("ParseEmbyWatchStates() error = %v", err)
	}

	assertWatchStates(t, got, []WatchState{{
		ExternalID:      ExternalID{Source: "tmdb", ID: "24680"},
		Watched:         true,
		PositionSeconds: 120,
		LastPlayed:      time.Date(2026, 6, 11, 1, 2, 3, 0, time.UTC),
	}})
}

func TestParseWatchStatesRejectsPlexDatabaseImport(t *testing.T) {
	_, err := ParsePlexWatchStates(strings.NewReader(`{}`))
	if err == nil {
		t.Fatal("ParsePlexWatchStates() error = nil, want unsupported error")
	}
}

// TestParseWatchStatesBoundsReadSize asserts an export exceeding
// maxWatchStatesBytes is rejected with an explicit too-large error instead of
// being decoded as unbounded JSON (mirrors TestParseOrderedListsBoundsReadSize
// in lists_test.go).
func TestParseWatchStatesBoundsReadSize(t *testing.T) {
	prefix := `{"Items": [{"ProviderIds": {"Tmdb": "` + strings.Repeat("x", maxWatchStatesBytes)
	input := strings.NewReader(prefix)

	got, err := ParseEmbyWatchStates(input)
	if !errors.Is(err, errWatchStatesTooLarge) {
		t.Fatalf("ParseEmbyWatchStates() = %#v, err %v, want errWatchStatesTooLarge", got, err)
	}
}

func assertWatchStates(t *testing.T, got, want []WatchState) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("watch state length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].LastPlayed.Equal(want[i].LastPlayed) {
			t.Fatalf("watch state[%d].LastPlayed = %v, want %v", i, got[i].LastPlayed, want[i].LastPlayed)
		}
		gotNoTime := got[i]
		wantNoTime := want[i]
		gotNoTime.LastPlayed = time.Time{}
		wantNoTime.LastPlayed = time.Time{}
		if gotNoTime != wantNoTime {
			t.Fatalf("watch state[%d] = %#v, want %#v; full result %#v", i, got[i], want[i], got)
		}
	}
}
