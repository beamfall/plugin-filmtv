package mediaimport

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
)

func FuzzParseNFO(f *testing.F) {
	for _, seed := range []string{
		`<movie><title>Example</title><year>2024</year><uniqueid type="tmdb">123</uniqueid></movie>`,
		`<episodedetails><title>Example</title><aired>2020-04-05</aired><id>tt1234567</id></episodedetails>`,
		`<movie><url>https://www.themoviedb.org/movie/456</url></movie>`,
		`<movie><title>broken`,
		``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		got, err := ParseNFO(bytes.NewReader(data))
		again, againErr := ParseNFO(bytes.NewReader(data))
		if mediaErrorText(err) != mediaErrorText(againErr) || !reflect.DeepEqual(got, again) {
			t.Fatalf("ParseNFO is not deterministic: (%+v, %v) then (%+v, %v)", got, err, again, againErr)
		}
		if err != nil {
			if !reflect.DeepEqual(got, ParseHint{}) {
				t.Fatalf("ParseNFO returned a partial hint with error: %+v, %v", got, err)
			}
			return
		}
		if got.Title != strings.TrimSpace(got.Title) {
			t.Fatalf("ParseNFO returned untrimmed title %q", got.Title)
		}
		if got.Year != 0 && (got.Year < 1900 || got.Year > 2099) {
			t.Fatalf("ParseNFO returned out-of-contract year %d", got.Year)
		}
		assertNormalizedIDs(t, got.ExternalIDs, true)
	})
}

func FuzzParseOrderedLists(f *testing.F) {
	for _, seed := range []string{
		`{"Collections":[{"Name":"Favorites","Items":[{"ProviderIds":{"Tmdb":"123"}}]}]}`,
		"#EXTM3U\n#PLAYLIST: Favorites\n#EXT-X-BEAMFALL-ID:tmdb:123\n",
		`{"Playlists":[]}`,
		`{`,
		``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		got, err := ParseOrderedLists(bytes.NewReader(data))
		again, againErr := ParseOrderedLists(bytes.NewReader(data))
		if mediaErrorText(err) != mediaErrorText(againErr) || !reflect.DeepEqual(got, again) {
			t.Fatalf("ParseOrderedLists is not deterministic: (%+v, %v) then (%+v, %v)", got, err, again, againErr)
		}
		if err != nil {
			if got != nil {
				t.Fatalf("ParseOrderedLists returned lists with error: %+v, %v", got, err)
			}
			return
		}
		if len(got) > len(data)+1 {
			t.Fatalf("ParseOrderedLists expanded %d input bytes into %d lists", len(data), len(got))
		}
		for index, list := range got {
			if list.Name != strings.TrimSpace(list.Name) {
				t.Fatalf("list %d has untrimmed name %q", index, list.Name)
			}
			if list.Name == "" && len(list.Items) == 0 {
				t.Fatalf("list %d is empty", index)
			}
			assertNormalizedIDs(t, list.Items, false)
		}
	})
}

func FuzzParseWatchStates(f *testing.F) {
	for _, seed := range []string{
		`[{"ProviderIds":{"Tmdb":"123"},"UserData":{"Played":true,"PlaybackPositionTicks":120000000,"LastPlayedDate":"2024-03-04T05:06:07Z"}}]`,
		`{"Items":[{"ProviderIds":{"Imdb":"tt123"},"UserData":{"LastPlayedDate":"invalid"}}]}`,
		`[]`,
		`{`,
		``,
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		jellyfin, jellyfinErr := ParseJellyfinWatchStates(bytes.NewReader(data))
		emby, embyErr := ParseEmbyWatchStates(bytes.NewReader(data))
		if mediaErrorText(jellyfinErr) != mediaErrorText(embyErr) || !reflect.DeepEqual(jellyfin, emby) {
			t.Fatalf("shared Emby/Jellyfin parser diverged: (%+v, %v) vs (%+v, %v)", jellyfin, jellyfinErr, emby, embyErr)
		}
		again, againErr := ParseJellyfinWatchStates(bytes.NewReader(data))
		if mediaErrorText(jellyfinErr) != mediaErrorText(againErr) || !reflect.DeepEqual(jellyfin, again) {
			t.Fatalf("watch-state parser is not deterministic: (%+v, %v) then (%+v, %v)", jellyfin, jellyfinErr, again, againErr)
		}
		if jellyfinErr != nil {
			if jellyfin != nil {
				t.Fatalf("watch-state parser returned states with error: %+v, %v", jellyfin, jellyfinErr)
			}
			return
		}
		if len(jellyfin) > len(data)+1 {
			t.Fatalf("watch-state parser expanded %d input bytes into %d states", len(data), len(jellyfin))
		}
		for index, state := range jellyfin {
			if state.ExternalID.Source == "" || state.ExternalID.ID == "" {
				t.Fatalf("state %d has empty external ID: %+v", index, state.ExternalID)
			}
			if state.ExternalID.Source != strings.ToLower(strings.TrimSpace(state.ExternalID.Source)) || state.ExternalID.ID != strings.TrimSpace(state.ExternalID.ID) {
				t.Fatalf("state %d has unnormalized external ID: %+v", index, state.ExternalID)
			}
		}
	})
}

func assertNormalizedIDs(t *testing.T, ids []ExternalID, unique bool) {
	t.Helper()
	seen := make(map[ExternalID]bool, len(ids))
	for _, id := range ids {
		if id.Source == "" || id.ID == "" || id.Source != strings.ToLower(strings.TrimSpace(id.Source)) || id.ID != strings.TrimSpace(id.ID) {
			t.Fatalf("unnormalized external ID: %+v", id)
		}
		if unique && seen[id] {
			t.Fatalf("duplicate external ID: %+v", id)
		}
		seen[id] = true
	}
}

func mediaErrorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
