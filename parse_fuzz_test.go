package filmtv

import (
	"bytes"
	"reflect"
	"strings"
	"testing"
	"time"

	sdk "github.com/beamfall/plugin-sdk"
)

func FuzzParseFilename(f *testing.F) {
	for _, seed := range []string{
		"The.Matrix.1999.1080p.mkv",
		"Some.Show.S02E05-E07.2020.WEB-DL.mkv",
		"Season 3 Episode 9.mp4",
		"KK.Movie.2003.mkv",
		"",
	} {
		f.Add(seed)
	}

	parser := New()
	f.Fuzz(func(t *testing.T, path string) {
		if len(path) > 64<<10 {
			return
		}
		got, ok := parser.Parse(path, sdk.ParseProbe{})
		again, againOK := parser.Parse(path, sdk.ParseProbe{})
		if ok != againOK || !reflect.DeepEqual(got, again) {
			t.Fatalf("Parse(%q) is not deterministic: (%+v, %v) then (%+v, %v)", path, got, ok, again, againOK)
		}
		if !ok {
			if !reflect.DeepEqual(got, sdk.ParsedMedia{}) {
				t.Fatalf("Parse(%q) returned non-zero refusal: %+v", path, got)
			}
			return
		}
		if got.Title == "" || got.Title != strings.TrimSpace(got.Title) {
			t.Fatalf("Parse(%q) returned invalid title %q", path, got.Title)
		}
		if got.Class != "movie" && got.Class != "episode" {
			t.Fatalf("Parse(%q) returned unknown class %q", path, got.Class)
		}
		if got.Year != 0 && (got.Year < 1900 || got.Year > 2099) {
			t.Fatalf("Parse(%q) returned out-of-contract year %d", path, got.Year)
		}
		if got.Class == "episode" && got.Sequence == "" {
			t.Fatalf("Parse(%q) returned an episode without a sequence: %+v", path, got)
		}
		if got.Index < 0 || got.Index > 999 || got.IndexEnd < 0 || got.IndexEnd > 999 {
			t.Fatalf("Parse(%q) returned an out-of-contract index: %+v", path, got)
		}
	})
}

func FuzzParseTraktCSV(f *testing.F) {
	for _, seed := range []string{
		"title,watched_at,tmdb,imdb,tvdb\nExample,2024-03-04T05:06:07Z,123,tt456,\n",
		"2024-03-04,123,tt456,789\n",
		"title,watched_at,tmdb\n\"quoted, title\",invalid,0\n",
		"",
	} {
		f.Add([]byte(seed))
	}

	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > 1<<20 {
			return
		}
		got, err := ParseTraktCSV(bytes.NewReader(data))
		again, againErr := ParseTraktCSV(bytes.NewReader(data))
		if errorText(err) != errorText(againErr) || !reflect.DeepEqual(got, again) {
			t.Fatalf("ParseTraktCSV is not deterministic: (%+v, %v) then (%+v, %v)", got, err, again, againErr)
		}
		if err != nil {
			if got != nil {
				t.Fatalf("ParseTraktCSV returned items with error: %+v, %v", got, err)
			}
			return
		}
		if len(got) > len(data)+1 {
			t.Fatalf("ParseTraktCSV expanded %d input bytes into %d rows", len(data), len(got))
		}
		for row, item := range got {
			if !item.WatchState.Present || !item.WatchState.Watched {
				t.Fatalf("row %d has invalid watch state: %+v", row, item.WatchState)
			}
			if !item.WatchState.OccurredAt.IsZero() && item.WatchState.OccurredAt.Location() != time.UTC {
				t.Fatalf("row %d timestamp is not UTC: %v", row, item.WatchState.OccurredAt)
			}
			seen := map[string]bool{}
			for _, id := range item.ExternalIDs {
				if id.Scheme != SchemeTMDB && id.Scheme != SchemeIMDB && id.Scheme != SchemeTVDB {
					t.Fatalf("row %d has unknown ID scheme %q", row, id.Scheme)
				}
				if id.Value == "" || id.Value == "0" || id.Value != strings.TrimSpace(id.Value) {
					t.Fatalf("row %d has invalid external ID %+v", row, id)
				}
				key := id.Scheme + "\x00" + id.Value
				if seen[key] {
					t.Fatalf("row %d has duplicate external ID %+v", row, id)
				}
				seen[key] = true
			}
		}
	})
}

func errorText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
