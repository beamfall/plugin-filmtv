package mediaimport

import (
	"strings"
	"testing"
)

func TestParseNFOExtractsUniqueIDsAndParseHints(t *testing.T) {
	input := strings.NewReader(`<?xml version="1.0"?>
<movie>
  <title>Example Item</title>
  <year>2024</year>
  <uniqueid type="tmdb" default="true">12345</uniqueid>
  <uniqueid type="imdb">tt7654321</uniqueid>
  <id>tt1111111</id>
  <tmdbid>67890</tmdbid>
  <url>https://www.themoviedb.org/movie/24680-example</url>
  <url>https://www.imdb.com/title/tt2222222/</url>
</movie>`)

	hint, err := ParseNFO(input)
	if err != nil {
		t.Fatalf("ParseNFO() error = %v", err)
	}

	if hint.Title != "Example Item" {
		t.Fatalf("Title = %q, want Example Item", hint.Title)
	}
	if hint.Year != 2024 {
		t.Fatalf("Year = %d, want 2024", hint.Year)
	}
	assertExternalIDs(t, hint.ExternalIDs, []ExternalID{
		{Source: "tmdb", ID: "12345"},
		{Source: "imdb", ID: "tt7654321"},
		{Source: "imdb", ID: "tt1111111"},
		{Source: "tmdb", ID: "67890"},
		{Source: "tmdb", ID: "24680"},
		{Source: "imdb", ID: "tt2222222"},
	})
}

func TestParseNFOHandlesCommonKodiItemShapes(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  ParseHint
	}{
		{
			name: "episodic root",
			input: `<episodedetails>
  <title>Chapter One</title>
  <aired>2023-03-04</aired>
  <uniqueid type="tvdb">777</uniqueid>
  <uniqueid type="tmdb">888</uniqueid>
</episodedetails>`,
			want: ParseHint{
				Title: "Chapter One",
				Year:  2023,
				ExternalIDs: []ExternalID{
					{Source: "tvdb", ID: "777"},
					{Source: "tmdb", ID: "888"},
				},
			},
		},
		{
			name: "container root",
			input: `<tvshow>
  <title>Example Container</title>
  <premiered>2022-10-09</premiered>
  <uniqueid type="tvdb">333</uniqueid>
  <url>https://thetvdb.com/series/example-container</url>
</tvshow>`,
			want: ParseHint{
				Title: "Example Container",
				Year:  2022,
				ExternalIDs: []ExternalID{
					{Source: "tvdb", ID: "333"},
					{Source: "tvdb", ID: "example-container"},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseNFO(strings.NewReader(tt.input))
			if err != nil {
				t.Fatalf("ParseNFO() error = %v", err)
			}
			if got.Title != tt.want.Title || got.Year != tt.want.Year {
				t.Fatalf("ParseNFO() = title %q year %d, want title %q year %d", got.Title, got.Year, tt.want.Title, tt.want.Year)
			}
			assertExternalIDs(t, got.ExternalIDs, tt.want.ExternalIDs)
		})
	}
}

// TestParseNFOIgnoresDateAddedAsYearHint asserts <dateadded> (a
// library-insertion timestamp, not a release date) is never used as a Year
// hint: an NFO with only <dateadded> must yield Year 0, not the insertion
// year.
func TestParseNFOIgnoresDateAddedAsYearHint(t *testing.T) {
	input := strings.NewReader(`<movie>
  <title>Old Film</title>
  <dateadded>2024-01-15 10:00:00</dateadded>
</movie>`)

	hint, err := ParseNFO(input)
	if err != nil {
		t.Fatalf("ParseNFO() error = %v", err)
	}
	if hint.Year != 0 {
		t.Fatalf("Year = %d, want 0 (dateadded must not be used as a year hint)", hint.Year)
	}
}

func TestParseNFORejectsMalformedXML(t *testing.T) {
	_, err := ParseNFO(strings.NewReader(`<movie><title>broken`))
	if err == nil {
		t.Fatal("ParseNFO() error = nil, want malformed XML error")
	}
}

func TestParseNFOExtractsURLOnlyProviderID(t *testing.T) {
	hint, err := ParseNFO(strings.NewReader("https://www.themoviedb.org/movie/603"))
	if err != nil {
		t.Fatalf("ParseNFO() error = %v", err)
	}
	assertExternalIDs(t, hint.ExternalIDs, []ExternalID{{Source: "tmdb", ID: "603"}})
}

func assertExternalIDs(t *testing.T, got, want []ExternalID) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("ExternalIDs length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("ExternalIDs[%d] = %#v, want %#v; full result %#v", i, got[i], want[i], got)
		}
	}
}
