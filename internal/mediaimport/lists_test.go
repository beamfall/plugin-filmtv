package mediaimport

import (
	"errors"
	"strings"
	"testing"
)

func TestParseOrderedListsFromJSONCollectionExport(t *testing.T) {
	input := strings.NewReader(`{
  "Collections": [
    {
      "Name": "Favorites",
      "Items": [
        {"ProviderIds": {"Tmdb": "101"}},
        {"ProviderIds": {"Imdb": "tt202"}}
      ]
    }
  ],
  "Playlists": [
    {
      "Name": "Queue",
      "Items": [
        {"ProviderIds": {"Tvdb": "303"}},
        {"ProviderIds": {"Tmdb": "404"}}
      ]
    }
  ]
}`)

	got, err := ParseOrderedLists(input)
	if err != nil {
		t.Fatalf("ParseOrderedLists() error = %v", err)
	}

	want := []OrderedList{
		{
			Name: "Favorites",
			Items: []ExternalID{
				{Source: "tmdb", ID: "101"},
				{Source: "imdb", ID: "tt202"},
			},
		},
		{
			Name: "Queue",
			Items: []ExternalID{
				{Source: "tvdb", ID: "303"},
				{Source: "tmdb", ID: "404"},
			},
		},
	}
	assertOrderedLists(t, got, want)
}

func TestParseOrderedListsFromM3UExtIDs(t *testing.T) {
	input := strings.NewReader(`#EXTM3U
#EXTINF:-1,Example A
#EXT-X-BEAMFALL-ID:tmdb:123
/library/example-a.mkv
#EXTINF:-1,Example B
#EXT-X-BEAMFALL-ID:imdb:tt456
/library/example-b.mkv
`)

	got, err := ParseOrderedLists(input)
	if err != nil {
		t.Fatalf("ParseOrderedLists() error = %v", err)
	}

	want := []OrderedList{{
		Items: []ExternalID{
			{Source: "tmdb", ID: "123"},
			{Source: "imdb", ID: "tt456"},
		},
	}}
	assertOrderedLists(t, got, want)
}

// TestParseOrderedListsBoundsReadSize asserts an export exceeding
// maxOrderedListsBytes is rejected with an explicit too-large error instead
// of being silently truncated and parsed as if it were a smaller, valid
// export.
func TestParseOrderedListsBoundsReadSize(t *testing.T) {
	prefix := `{"Collections": [{"Name": "` + strings.Repeat("x", maxOrderedListsBytes)
	input := strings.NewReader(prefix)

	got, err := ParseOrderedLists(input)
	if !errors.Is(err, errOrderedListsImportTooLarge) {
		t.Fatalf("ParseOrderedLists() = %#v, err %v, want errOrderedListsImportTooLarge", got, err)
	}
}

// TestParseOrderedListsAllowsRealisticLargeExport asserts a large collection
// export (well past the old 64 KB cap, under the new 64 MB cap) is parsed in
// full rather than truncated.
func TestParseOrderedListsAllowsRealisticLargeExport(t *testing.T) {
	var b strings.Builder
	b.WriteString(`{"Collections": [{"Name": "Favorites", "Items": [`)
	const items = 3000 // several hundred KB of JSON, well under 64 MB
	for i := 0; i < items; i++ {
		if i > 0 {
			b.WriteString(",")
		}
		b.WriteString(`{"ProviderIds": {"Tmdb": "101"}}`)
	}
	b.WriteString(`]}]}`)

	got, err := ParseOrderedLists(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("ParseOrderedLists returned error: %v", err)
	}
	if len(got) != 1 || len(got[0].Items) != items {
		t.Fatalf("got %d lists (items[0]=%d), want 1 list with %d items", len(got), len(got[0].Items), items)
	}
}

// TestParseOrderedListsM3ULongLineDoesNotTruncate asserts a single M3U line
// longer than bufio.Scanner's 64 KB default token cap does not silently end
// the scan: entries after the long line must still be parsed (the scanner's
// buffer is raised to the already-bounded input size in parseM3UOrderedList).
func TestParseOrderedListsM3ULongLineDoesNotTruncate(t *testing.T) {
	longComment := "#EXTINF:-1," + strings.Repeat("x", 128*1024)
	input := strings.NewReader("#EXTM3U\n" +
		"#EXT-X-BEAMFALL-ID:tmdb:123\n" +
		longComment + "\n" +
		"#EXT-X-BEAMFALL-ID:imdb:tt456\n")

	got, err := ParseOrderedLists(input)
	if err != nil {
		t.Fatalf("ParseOrderedLists() error = %v", err)
	}

	want := []OrderedList{{
		Items: []ExternalID{
			{Source: "tmdb", ID: "123"},
			{Source: "imdb", ID: "tt456"},
		},
	}}
	assertOrderedLists(t, got, want)
}

func assertOrderedLists(t *testing.T, got, want []OrderedList) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("list length = %d, want %d: %#v", len(got), len(want), got)
	}
	for i := range want {
		if got[i].Name != want[i].Name {
			t.Fatalf("list[%d].Name = %q, want %q", i, got[i].Name, want[i].Name)
		}
		assertExternalIDs(t, got[i].Items, want[i].Items)
	}
}
