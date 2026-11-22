package filmtv_test

import (
	"encoding/json"
	"net/url"
	"os"
	"slices"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"
)

type childRow struct {
	key, label, sequence, index, year, artwork string
}

func TestTMDBTVChildListings(t *testing.T) {
	scraper := scraperByID(t, "tmdb-tv")
	if len(scraper.Children) != 2 {
		t.Fatalf("tmdb-tv has %d child levels, want 2", len(scraper.Children))
	}
	season, episode := scraper.Children[0], scraper.Children[1]
	if season.Level != "season" || episode.Level != "episode" {
		t.Fatalf("TMDB child levels = %q, %q, want season, episode", season.Level, episode.Level)
	}

	const artwork = "https://image.tmdb.org/t/p/original"
	seasons := checkChildRows(t, scraper, season, "tmdb-tv-seasons.json", []childRow{
		{"0", "Specials", "0", "", "2023", artwork + "/YELUBKBQUB8Ba0pLDQpyv74JkRC.jpg"},
		{"1", "Season 1", "1", "", "2024", artwork + "/cRFai12ZfLS8tWxrSo2NSmbIoTK.jpg"},
		{"2", "Season 2", "2", "", "2025", artwork + "/WSlrgljT4OqmzVpUvlPqlx8yHF2.jpg"},
	})
	checkChildRows(t, scraper, episode, "tmdb-tv-season-1.json", []childRow{
		{"1", "Le retour", "1", "1", "2024", artwork + "/qavbgM4MQY37kxfAeg3h2Vdk86N.jpg"},
		{"2", "La marée haute", "1", "2", "2024", artwork + "/vrrYE9M2ezwbsPwA6LKsfNbFMko.jpg"},
		{"3", "Les lettres", "1", "3", "2024", artwork + "/GjV1mFeOSv49fQnC3PpLugRoIHr.jpg"},
	})

	const candidateID = "4242"
	const showURL = "https://api.themoviedb.org/3/tv/4242"
	if got := scraper.Endpoint + strings.ReplaceAll(season.ListTemplate, "{id}", candidateID); got != showURL {
		t.Errorf("season list URL = %q, want %q", got, showURL)
	}
	for i, want := range []string{showURL + "/season/0", showURL + "/season/1", showURL + "/season/2"} {
		parent := seasons[i].key
		if !sdk.ValidChildKeySegment(parent) {
			t.Fatalf("season key %q cannot be used as a path segment", parent)
		}
		replace := strings.NewReplacer("{id}", candidateID, "{parent}", url.PathEscape(parent))
		if got := scraper.Endpoint + replace.Replace(episode.ListTemplate); got != want {
			t.Errorf("episode list URL = %q, want %q", got, want)
		}
	}
}

func TestTVDBSeriesChildListings(t *testing.T) {
	scraper := scraperByID(t, "tvdb-series")
	if len(scraper.Children) != 2 {
		t.Fatalf("tvdb-series has %d child levels, want 2", len(scraper.Children))
	}
	season, episode := scraper.Children[0], scraper.Children[1]
	if season.Level != "season" || episode.Level != "episode" {
		t.Fatalf("TVDB child levels = %q, %q, want season, episode", season.Level, episode.Level)
	}

	const artwork = "https://artworks.thetvdb.com/banners/"
	seasons := checkChildRows(t, scraper, season, "tvdb-series-seasons.json", []childRow{
		{"2064401", "Specials", "0", "", "2022", artwork + "seasons/2064401.jpg"},
		{"2064402", "Season 1", "1", "", "2023", artwork + "seasons/2064402.jpg"},
		{"2064403", "Season 2", "2", "", "2024", artwork + "seasons/2064403.jpg"},
	})
	if got, want := strings.ReplaceAll(season.ListTemplate, "{id}", "432087"), "/series/432087/extended"; got != want {
		t.Errorf("season list path = %q, want %q", got, want)
	}
	parent := url.PathEscape(seasons[1].key)
	if got, want := strings.ReplaceAll(episode.ListTemplate, "{parent}", parent), "/seasons/2064402/extended"; got != want {
		t.Errorf("episode list path = %q, want %q", got, want)
	}
	checkChildRows(t, scraper, episode, "tvdb-season-episodes.json", []childRow{
		{"9670401", "The Breakwater", "1", "1", "2023", artwork + "episodes/432087/9670401.jpg"},
		{"9670402", "A Falling Tide", "1", "2", "2023", artwork + "episodes/432087/9670402.jpg"},
		{"9670403", "The Keeper's Log", "1", "3", "2023", artwork + "episodes/432087/9670403.jpg"},
	})
}

func TestTVmazeShowSearch(t *testing.T) {
	scraper := scraperByID(t, "tvmaze-show")
	// Trimmed response from https://api.tvmaze.com/search/shows?q=game+of+thrones.
	trace := sdktest.CompileScraperTrace(scraper, sdktest.TraceInput{
		Query:       "game of thrones",
		SearchArray: loadArrayFixture(t, "tvmaze-search-shows.json"),
	})
	if len(trace.Requests) != 1 {
		t.Fatalf("search returned %d requests, want 1", len(trace.Requests))
	}
	request := trace.Requests[0]
	target, err := url.Parse(request.URL)
	if err != nil || request.Method != "GET" || target.Host != "api.tvmaze.com" ||
		target.Path != "/search/shows" || target.Query().Get("q") != "game of thrones" {
		t.Errorf("search request = %+v, want GET /search/shows with the candidate query", request)
	}
	want := []struct{ id, title, year, network, poster string }{
		{"82", "Game of Thrones", "2011", "HBO", "https://static.tvmaze.com/uploads/images/original_untouched/498/1245274.jpg"},
		{"42281", "Game of Thrones: Inside the Episode", "2019", "HBO", ""},
	}
	if len(trace.Candidates) != len(want) {
		t.Fatalf("search returned %d candidates, want %d", len(trace.Candidates), len(want))
	}
	for i, candidate := range trace.Candidates {
		match := want[i]
		if candidate.ID != match.id || candidate.Fields["title"] != match.title ||
			candidate.Fields["year"] != match.year || candidate.Fields["media.network"] != match.network ||
			candidate.Artwork["poster"] != match.poster {
			t.Errorf("candidate %d = %+v, want %+v", i, candidate, match)
		}
	}
}

func TestTVmazeShowChildListings(t *testing.T) {
	scraper := scraperByID(t, "tvmaze-show")
	if len(scraper.Children) != 2 {
		t.Fatalf("tvmaze-show has %d child levels, want 2", len(scraper.Children))
	}
	season, episode := scraper.Children[0], scraper.Children[1]
	if season.Level != "season" || episode.Level != "episode" {
		t.Fatalf("TVmaze child levels = %q, %q, want season, episode", season.Level, episode.Level)
	}

	const artwork = "https://static.tvmaze.com/uploads/images/"
	seasons := checkChildRows(t, scraper, season, "tvmaze-show-seasons.json", []childRow{
		{"123401", "The Crossing", "1", "", "2024", artwork + "medium_portrait/542/1355375.jpg"},
		{"123402", "", "2", "", "2025", artwork + "medium_portrait/633/1582712.jpg"},
	})
	if got, want := strings.ReplaceAll(season.ListTemplate, "{id}", "82"), "/shows/82/seasons"; got != want {
		t.Errorf("season list path = %q, want %q", got, want)
	}
	parent := url.PathEscape(seasons[0].key)
	if got, want := strings.ReplaceAll(episode.ListTemplate, "{parent}", parent), "/seasons/123401/episodes"; got != want {
		t.Errorf("episode list path = %q, want %q", got, want)
	}
	checkChildRows(t, scraper, episode, "tvmaze-season-episodes.json", []childRow{
		{"2846101", "Night Ferry", "1", "1", "2024", artwork + "medium_landscape/542/1355376.jpg"},
		{"2846102", "The Island Road", "1", "2", "2024", artwork + "medium_landscape/542/1355377.jpg"},
		{"2846103", "Christmas on the Sound", "1", "", "2024", artwork + "medium_landscape/550/1375378.jpg"},
	})
}

func TestAniListHasNoChildListings(t *testing.T) {
	for _, id := range []string{"anilist-series", "anilist-movie"} {
		if children := scraperByID(t, id).Children; len(children) != 0 {
			t.Errorf("%s declares child listings: %v", id, children)
		}
	}
}

func checkChildRows(t *testing.T, scraper sdk.ScraperSpec, level sdk.ChildListing, fixture string, want []childRow) []childRow {
	t.Helper()
	// The SDK trace resolves candidate arrays and scalar fields with the same
	// path dialect as ChildListing. Feed it the declared paths for this level.
	spec := sdk.ScraperSpec{
		SearchTemplate:  level.ListTemplate,
		CandidatesPath:  level.ItemsPath,
		CandidateIDPath: level.Key,
		SearchFieldMap: map[string]string{
			"label": level.Label, "sequence": level.Sequence, "index": level.Index, "year": level.Year,
		},
		Transforms:       map[string]string{"year": "year-from-date"},
		ArtworkBaseURL:   scraper.ArtworkBaseURL,
		SearchArtworkMap: map[string]string{"image": level.Artwork},
	}
	var input sdktest.TraceInput
	if level.ItemsPath == "$" {
		input.SearchArray = loadArrayFixture(t, fixture)
	} else {
		input.SearchDoc = loadFixture(t, fixture)
	}
	trace := sdktest.CompileScraperTrace(spec, input)
	rows := make([]childRow, 0, len(trace.Candidates))
	for _, candidate := range trace.Candidates {
		rows = append(rows, childRow{
			key:      candidate.ID,
			label:    candidate.Fields["label"],
			sequence: candidate.Fields["sequence"],
			index:    candidate.Fields["index"],
			year:     candidate.Fields["year"],
			artwork:  candidate.Artwork["image"],
		})
	}
	if !slices.Equal(rows, want) {
		t.Fatalf("%s rows = %+v, want %+v", level.Level, rows, want)
	}
	return rows
}

func loadArrayFixture(t *testing.T, name string) []any {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var doc []any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}
