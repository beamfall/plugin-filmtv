package filmtv_test

import (
	"encoding/json"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	filmtv "github.com/beamfall/plugin-filmtv"
)

func scraperByID(t *testing.T, id string) sdk.ScraperSpec {
	t.Helper()
	for _, scraper := range filmtv.Bundle().Scrapers {
		if scraper.ID == id {
			return scraper
		}
	}
	t.Fatalf("scraper %s not found", id)
	return sdk.ScraperSpec{}
}

// loadFixture decodes a recorded document. The fixtures keep each source's
// response shape with synthetic values; tests never reach the network.
func loadFixture(t *testing.T, name string) map[string]any {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(data, &doc); err != nil {
		t.Fatal(err)
	}
	return doc
}

// TestTVmazeTermsAreHonored: TVmaze needs no key, licenses its data
// CC BY-SA 4.0 with a linked credit, and allows at least 20 calls per 10 s
// (tvmaze.com/api, read 2026-11-14).
func TestTVmazeTermsAreHonored(t *testing.T) {
	scraper := scraperByID(t, "tvmaze-show")
	if scraper.AuthRef != "" {
		t.Errorf("tvmaze-show AuthRef = %q, want none", scraper.AuthRef)
	}
	if scraper.Attribution == nil {
		t.Fatal("tvmaze-show declares no attribution")
	}
	if scraper.Attribution.URL != "https://www.tvmaze.com" {
		t.Errorf("tvmaze-show attribution link = %q", scraper.Attribution.URL)
	}
	text := scraper.Attribution.Text.Resolve("")
	if !strings.Contains(text, "TVmaze") || !strings.Contains(text, "CC BY-SA 4.0") {
		t.Errorf("tvmaze-show attribution %q must credit TVmaze and its licence", text)
	}
	if scraper.MinRequestIntervalMS < 500 {
		t.Errorf("tvmaze-show MinRequestIntervalMS = %d, want at least 500", scraper.MinRequestIntervalMS)
	}
	want := map[string]string{"tvdb": "/lookup/shows?thetvdb={id}", "imdb": "/lookup/shows?imdb={id}"}
	if len(scraper.LookupByIDTemplate) != len(want) {
		t.Errorf("tvmaze-show LookupByIDTemplate = %#v, want %#v", scraper.LookupByIDTemplate, want)
	}
	for key, tmpl := range want {
		if scraper.LookupByIDTemplate[key] != tmpl {
			t.Errorf("tvmaze-show %s lookup = %q, want %q", key, scraper.LookupByIDTemplate[key], tmpl)
		}
	}
}

// TestAniListNeedsOperatorToken: AniList is reached with each operator's own
// access token, so without one the scrapers are omitted and send nothing; its
// current limit is 30 requests per minute (docs.anilist.co, read 2026-11-14).
// Core spaces requests per scraper ID, so the two scrapers' rates add up.
func TestAniListNeedsOperatorToken(t *testing.T) {
	perMinute := 0
	for id, class := range map[string]string{"anilist-series": "episode", "anilist-movie": "movie"} {
		scraper := scraperByID(t, id)
		if scraper.AuthRef != "anilist-access-token" {
			t.Errorf("%s AuthRef = %q, want anilist-access-token", id, scraper.AuthRef)
		}
		if !scraper.OptionalAuth {
			t.Errorf("%s must be optional-auth so an operator without a token sends nothing", id)
		}
		if scraper.AuthStyle != "" && scraper.AuthStyle != "bearer" {
			t.Errorf("%s AuthStyle = %q, want bearer", id, scraper.AuthStyle)
		}
		if scraper.MinRequestIntervalMS <= 0 {
			t.Fatalf("%s declares no request spacing", id)
		}
		perMinute += 60000 / scraper.MinRequestIntervalMS
		if got := scraper.ImpliedMediaFields["media.class"]; got != class {
			t.Errorf("%s implies class %q, want %q", id, got, class)
		}
	}
	if perMinute > 30 {
		t.Errorf("AniList scrapers together allow %d requests per minute, want at most 30", perMinute)
	}
}

// TestAnimeScraperTraces grades the new scrapers' requests and mappings
// against recorded documents through the SDK's reference trace.
func TestAnimeScraperTraces(t *testing.T) {
	bearer := sdktest.AuthPlacement{Location: "header", Name: "Authorization", Scheme: "Bearer"}
	series := scraperByID(t, "anilist-series")
	movie := scraperByID(t, "anilist-movie")
	sdktest.RunScraperTraceConformance(t, []sdktest.ScraperTraceVector{
		{
			Name:  "tvmaze lookup by id",
			Spec:  scraperByID(t, "tvmaze-show"),
			Input: sdktest.TraceInput{ID: "82", LookupDoc: loadFixture(t, "tvmaze-show-episodes.json")},
			Want: sdktest.ScraperTrace{
				Requests: []sdktest.TraceRequest{{Role: "lookup", Method: "GET",
					URL: "https://api.tvmaze.com/shows/82?embed=episodes"}},
				Fields: map[string]string{
					"title":         "Pioneer One",
					"year":          "2010",
					"media.network": "Example Network",
				},
				Artwork: map[string]string{
					"poster": "https://static.tvmaze.com/uploads/images/original_untouched/1/1.jpg",
				},
			},
		},
		{
			Name: "anilist series search and lookup",
			Spec: series,
			Input: sdktest.TraceInput{Query: "Pioneer One", ID: "101",
				SearchDoc: loadFixture(t, "anilist-search.json"),
				LookupDoc: loadFixture(t, "anilist-lookup.json")},
			Want: sdktest.ScraperTrace{
				Requests: []sdktest.TraceRequest{
					{Role: "search", Page: 1, Method: "POST", URL: "https://graphql.anilist.co",
						Body: strings.ReplaceAll(series.SearchTemplate, "{query}", "Pioneer One"), Auth: bearer},
					{Role: "lookup", Method: "POST", URL: "https://graphql.anilist.co",
						Body: strings.ReplaceAll(series.LookupTemplate, "{id}", "101"), Auth: bearer},
				},
				Fields: map[string]string{
					"title":                "Pioneer One",
					"year":                 "2010",
					"popularity":           "5120",
					"media.original-title": "パイオニア・ワン",
				},
				Artwork: map[string]string{
					"poster":   "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx101.jpg",
					"backdrop": "https://s4.anilist.co/file/anilistcdn/media/anime/banner/101.jpg",
				},
				Candidates: []sdktest.TraceCandidate{
					{ID: "101",
						Fields:  map[string]string{"title": "Pioneer One", "year": "2010", "popularity": "5120"},
						Artwork: map[string]string{"poster": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx101.jpg"}},
					{ID: "102",
						Fields:  map[string]string{"title": "Pioneer One Season 2", "year": "2011", "popularity": "3010"},
						Artwork: map[string]string{"poster": "https://s4.anilist.co/file/anilistcdn/media/anime/cover/large/bx102.jpg"}},
				},
			},
		},
		{
			Name:  "anilist movie search body",
			Spec:  movie,
			Input: sdktest.TraceInput{Query: "Pioneer One"},
			Want: sdktest.ScraperTrace{
				Requests: []sdktest.TraceRequest{{Role: "search", Page: 1, Method: "POST",
					URL:  "https://graphql.anilist.co",
					Body: strings.ReplaceAll(movie.SearchTemplate, "{query}", "Pioneer One"), Auth: bearer}},
			},
		},
	})
}

// TestAniListQueriesSelectFormats pins the format split between the two
// AniList scrapers and that every mapped path is selected by its query.
func TestAniListQueriesSelectFormats(t *testing.T) {
	series := scraperByID(t, "anilist-series")
	movie := scraperByID(t, "anilist-movie")
	if !strings.Contains(series.SearchTemplate, "format_in: [TV, TV_SHORT, ONA, OVA, SPECIAL]") {
		t.Errorf("anilist-series search formats: %s", series.SearchTemplate)
	}
	if !strings.Contains(movie.SearchTemplate, "format_in: [MOVIE]") {
		t.Errorf("anilist-movie search formats: %s", movie.SearchTemplate)
	}
	for _, scraper := range []sdk.ScraperSpec{series, movie} {
		if !strings.Contains(scraper.SearchTemplate, `search: "{query}"`) {
			t.Errorf("%s search must pass {query} as a string value", scraper.ID)
		}
		for _, field := range []string{"bannerImage", "genres", "studios(isMain: true) { nodes { name } }", "native"} {
			if !strings.Contains(scraper.LookupTemplate, field) {
				t.Errorf("%s lookup query does not select %s", scraper.ID, field)
			}
		}
	}
}

// TestAnimeEntityMaps pins genre and studio hydration from the lookup
// documents.
func TestAnimeEntityMaps(t *testing.T) {
	want := map[string][]sdk.EntityMap{
		"tvmaze-show": {{Kind: "genre", Relation: "genre-of", Path: "genres"}},
		"anilist-series": {
			{Kind: "genre", Relation: "genre-of", Path: "Media.genres"},
			{Kind: "studio", Relation: "produced", Path: "Media.studios.nodes", NamePath: "name"},
		},
	}
	for id, maps := range want {
		got := scraperByID(t, id).EntityMaps
		if len(got) != len(maps) {
			t.Errorf("%s EntityMaps = %#v, want %#v", id, got, maps)
			continue
		}
		for i := range maps {
			if got[i].Kind != maps[i].Kind || got[i].Relation != maps[i].Relation ||
				got[i].Path != maps[i].Path || got[i].NamePath != maps[i].NamePath {
				t.Errorf("%s EntityMaps[%d] = %#v, want %#v", id, i, got[i], maps[i])
			}
		}
	}
}

// TestTMDBTVShowContainerTrace freezes the tmdb-tv show container output for
// a recorded series document (CORE-2514, CORE-2513): the show's details,
// poster and backdrop, the TMDB title with its apostrophes and accents in
// place of the folder spelling, and links to the genres and the cast with a
// person id, in billing order, named by their /person/{id} documents. The season map's {sequence} path has no
// offline value, so it emits nothing here.
func TestTMDBTVShowContainerTrace(t *testing.T) {
	const image = "https://image.tmdb.org/t/p/original"
	sdktest.RunScraperTraceConformance(t, []sdktest.ScraperTraceVector{{
		Name: "tmdb-tv show container",
		Spec: scraperByID(t, "tmdb-tv"),
		Input: sdktest.TraceInput{ID: "4242", LookupDoc: loadFixture(t, "tmdb-tv-series.json"),
			EntityDocs: map[string]map[string]any{
				"/person/101": {"name": "Ada Lovelace-Byron"},
				"/person/102": {"name": "Zoë O'Brien"},
			}},
		Want: sdktest.ScraperTrace{
			Requests: []sdktest.TraceRequest{{Role: "lookup", Method: "GET",
				URL:  "https://api.themoviedb.org/3/tv/4242?append_to_response=credits,content_ratings",
				Auth: sdktest.AuthPlacement{Location: "query", Name: "api_key", InURL: true}}},
			Fields: map[string]string{
				"title":                        "L'Été d'Amélie",
				"summary":                      "A synthetic series document in the TMDB /tv/{id} shape.",
				"year":                         "2024",
				"media.original-title":         "L'Été d'Amélie",
				"media.production-status":      "Returning Series",
				"media.tagline":                "",
				"media.network":                "Netflix",
				"media.certification-display":  "TV-MA",
				"media.community-rating":       "7.5",
				"media.community-rating-votes": "1210",
			},
			Artwork: map[string]string{
				"poster":   image + "/series-poster.jpg",
				"backdrop": image + "/series-backdrop.jpg",
			},
			Containers: []sdktest.TraceContainer{{
				Kind:  "show",
				Title: "L'Été d'Amélie",
				Fields: map[string]string{
					"summary":                        "A synthetic series document in the TMDB /tv/{id} shape.",
					"first-air-date":                 "2024-03-21",
					"last-air-date":                  "2024-03-21",
					"status":                         "Returning Series",
					"network":                        "Netflix",
					"certification-display":          "TV-MA",
					filmtv.CommunityRatingField:      "7.5",
					filmtv.CommunityRatingVotesField: "1210",
				},
				Artwork: map[string]string{
					"poster":   image + "/series-poster.jpg",
					"backdrop": image + "/series-backdrop.jpg",
				},
				Links: []sdktest.TraceLink{
					{Relation: "credit", Kind: "person", Name: "Ada Lovelace-Byron"},
					{Relation: "credit", Kind: "person", Name: "Zoë O'Brien"},
					{Relation: "in-genre", Kind: "genre", Name: "Drama"},
					{Relation: "in-genre", Kind: "genre", Name: "Sci-Fi & Fantasy"},
				},
			}},
		},
	}})
}

// TestTMDBCreditsTrace exercises tmdb-tv's cast lookup cap (10 /person/{id}
// docs, skipping id-less entries without burning slots, hydrating the 11th+
// cast element directly) and key-crew ElementFilter through the SDK trace.
func TestTMDBCreditsTrace(t *testing.T) {
	cast := make([]any, 0, 12)
	entityDocs := map[string]map[string]any{}
	wantLinks := make([]sdktest.TraceLink, 0, 17)
	for i := 1; i <= 10; i++ {
		id := fmt.Sprintf("%d", 200+i)
		resolved := fmt.Sprintf("Resolved Cast %02d", i)
		cast = append(cast, map[string]any{
			"id":           float64(200 + i),
			"name":         fmt.Sprintf("Cast %02d", i),
			"character":    fmt.Sprintf("Role %02d", i),
			"order":        float64(i - 1),
			"profile_path": fmt.Sprintf("/cast-%02d.jpg", i),
		})
		entityDocs["/person/"+id] = map[string]any{
			"name":         resolved,
			"profile_path": fmt.Sprintf("/cast-%02d.jpg", i),
		}
		wantLinks = append(wantLinks, sdktest.TraceLink{Relation: "credit", Kind: "person", Name: resolved})
	}
	// An id-less entry before the lookup cap is skipped without spending a slot.
	cast = slices.Insert(cast, 2, any(map[string]any{"name": "Uncredited Extra", "order": float64(99)}))
	// Past MaxLookups (10), cast hydrates straight from the element itself even when a /person doc exists.
	cast = append(cast, map[string]any{
		"id":           float64(211),
		"name":         "Cast 11 Past Cap",
		"character":    "Role 11",
		"order":        float64(10),
		"profile_path": "/cast-11.jpg",
	})
	entityDocs["/person/211"] = map[string]any{
		"name":         "Should Not Fetch 11",
		"profile_path": "/cast-11.jpg",
	}
	wantLinks = append(wantLinks, sdktest.TraceLink{Relation: "credit", Kind: "person", Name: "Cast 11 Past Cap"})

	crew := []any{
		map[string]any{"name": "Filtered Casting", "job": "Casting", "department": "Production", "profile_path": "/casting.jpg"},
		map[string]any{"name": "Crew Director", "job": "Director", "department": "Directing", "profile_path": "/dir.jpg"},
		map[string]any{"name": "Crew Screenplay", "job": "Screenplay", "department": "Writing", "profile_path": "/screen.jpg"},
		map[string]any{"name": "Crew Writer", "job": "Writer", "department": "Writing", "profile_path": "/writer.jpg"},
		map[string]any{"name": "Crew Producer", "job": "Producer", "department": "Production", "profile_path": "/prod.jpg"},
		map[string]any{"name": "Crew Composer", "job": "Original Music Composer", "department": "Sound", "profile_path": "/music.jpg"},
		map[string]any{"name": "Crew DP", "job": "Director of Photography", "department": "Camera", "profile_path": "/dp.jpg"},
		map[string]any{"name": "Filtered Grip", "job": "Key Grip", "department": "Camera", "profile_path": "/grip.jpg"},
	}
	for _, name := range []string{"Crew Director", "Crew Screenplay", "Crew Writer", "Crew Producer", "Crew Composer", "Crew DP"} {
		wantLinks = append(wantLinks, sdktest.TraceLink{Relation: "credit", Kind: "person", Name: name})
	}

	sdktest.RunScraperTraceConformance(t, []sdktest.ScraperTraceVector{{
		Name: "tmdb-tv cast cap and key crew filter",
		Spec: scraperByID(t, "tmdb-tv"),
		Input: sdktest.TraceInput{
			ID: "9001",
			LookupDoc: map[string]any{
				"id":   float64(9001),
				"name": "Credit Roster",
				"credits": map[string]any{
					"cast": cast,
					"crew": crew,
				},
			},
			EntityDocs: entityDocs,
		},
		Want: sdktest.ScraperTrace{
			Requests: []sdktest.TraceRequest{{Role: "lookup", Method: "GET",
				URL:  "https://api.themoviedb.org/3/tv/9001?append_to_response=credits,content_ratings",
				Auth: sdktest.AuthPlacement{Location: "query", Name: "api_key", InURL: true}}},
			Fields: map[string]string{"title": "Credit Roster"},
			Containers: []sdktest.TraceContainer{{
				Kind:   "show",
				Title:  "Credit Roster",
				Fields: map[string]string{},
				Links:  wantLinks,
			}},
		},
	}})
}
