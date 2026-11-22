package filmtv_test

import (
	"net/url"
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
	trace := sdktest.CompileScraperTrace(spec, sdktest.TraceInput{SearchDoc: loadFixture(t, fixture)})
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
