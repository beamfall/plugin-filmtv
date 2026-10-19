package filmtv

import (
	"errors"
	"strings"
	"testing"
	"time"
)

func TestParseTraktCSV(t *testing.T) {
	input := strings.NewReader(`title,watched_at,tmdb,imdb,tvdb
Example Movie,2024-03-04T05:06:07Z,12345,tt7654321,
Example Episode,2024-03-05 06:07:08,,,98765
`)
	got, err := ParseTraktCSV(input)
	if err != nil {
		t.Fatalf("ParseTraktCSV returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d rows, want 2", len(got))
	}
	first := got[0]
	if !first.WatchState.Present || !first.WatchState.Watched {
		t.Fatalf("first watch state = %+v, want watched", first.WatchState)
	}
	want := time.Date(2024, 3, 4, 5, 6, 7, 0, time.UTC)
	if !first.WatchState.OccurredAt.Equal(want) {
		t.Fatalf("first watched_at = %v, want %v", first.WatchState.OccurredAt, want)
	}
	if len(first.ExternalIDs) != 2 || first.ExternalIDs[0].Scheme != SchemeTMDB || first.ExternalIDs[0].Value != "12345" || first.ExternalIDs[1].Scheme != SchemeIMDB {
		t.Fatalf("first external ids = %#v", first.ExternalIDs)
	}
	second := got[1]
	if len(second.ExternalIDs) != 1 || second.ExternalIDs[0].Scheme != SchemeTVDB || second.ExternalIDs[0].Value != "98765" {
		t.Fatalf("second external ids = %#v, want tvdb 98765", second.ExternalIDs)
	}
}

// TestParseTraktCSVBoundsReadSize asserts an export exceeding maxTraktCSVBytes
// is rejected with an explicit too-large error instead of being silently
// truncated and parsed as if it were a smaller, valid export.
func TestParseTraktCSVBoundsReadSize(t *testing.T) {
	header := "title,watched_at,tmdb,imdb,tvdb\n"
	oversizedRow := strings.Repeat("x", maxTraktCSVBytes)

	got, err := ParseTraktCSV(strings.NewReader(header + oversizedRow))
	if !errors.Is(err, errTraktImportTooLarge) {
		t.Fatalf("ParseTraktCSV() = %#v, err %v, want errTraktImportTooLarge", got, err)
	}
}

// TestParseTraktCSVAllowsRealisticLargeExport asserts a real-world-sized
// Trakt watched-history export (well past the old 64 KB cap, comfortably
// under the new 64 MB cap) is parsed in full rather than truncated.
func TestParseTraktCSVAllowsRealisticLargeExport(t *testing.T) {
	var b strings.Builder
	b.WriteString("title,watched_at,tmdb,imdb,tvdb\n")
	const rows = 5000 // ~64 KB+ of CSV, well under 64 MB
	for i := 0; i < rows; i++ {
		b.WriteString("Example Movie,2024-03-04T05:06:07Z,12345,tt7654321,\n")
	}

	got, err := ParseTraktCSV(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ParseTraktCSV returned error: %v", err)
	}
	if len(got) != rows {
		t.Fatalf("got %d rows, want %d", len(got), rows)
	}
}
