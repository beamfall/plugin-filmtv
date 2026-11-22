package filmtv_test

import (
	"testing"

	sdk "github.com/beamfall/plugin-sdk"
	"github.com/beamfall/plugin-sdk/sdktest"

	filmtv "github.com/beamfall/plugin-filmtv"
)

// TestTMDBMovieFilmSeriesMaps pins the film-series build from TMDB's
// belongs_to_collection (CORE-3152): film-series holds each film through a
// single-holder relation, the tmdb-movie scraper's series map creates the
// series (name, tmdb-id, poster) and links the film, and its container map adds
// the backdrop. The TV scraper builds no series.
func TestTMDBMovieFilmSeriesMaps(t *testing.T) {
	var series sdk.EntityKind
	for _, kind := range filmtv.Bundle().EntityKinds {
		if kind.Name == "film-series" {
			series = kind
		}
	}
	if series.Archetype != "container" || !series.OrderedChildren || len(series.Relations) != 1 ||
		series.Relations[0].Name != filmtv.FilmSeriesFilmRelation || series.Relations[0].ToKind != sdk.MediaKindName ||
		series.Relations[0].Cardinality != "one" {
		t.Fatalf("film-series kind = %+v, want an ordered container holding one film per relation", series)
	}

	var link, container []sdk.EntityMap
	for _, em := range scraperByID(t, "tmdb-movie").EntityMaps {
		if em.Kind != "film-series" {
			continue
		}
		if em.Container {
			container = append(container, em)
		} else {
			link = append(link, em)
		}
	}
	if len(link) != 1 || link[0].Relation != filmtv.FilmSeriesFilmRelation || link[0].Path != "belongs_to_collection" ||
		link[0].NamePath != "name" || link[0].ArtworkPath != "poster_path" || link[0].FieldMap["tmdb-id"] != "id" {
		t.Errorf("tmdb-movie series map = %+v, want belongs_to_collection name, poster_path and tmdb-id=id", link)
	}
	if len(container) != 1 || container[0].Path != "belongs_to_collection" ||
		container[0].ArtworkMap["backdrop"] != "backdrop_path" {
		t.Errorf("tmdb-movie series container map = %+v, want the belongs_to_collection backdrop", container)
	}
	for _, em := range scraperByID(t, "tmdb-tv").EntityMaps {
		if em.Kind == "film-series" {
			t.Errorf("tmdb-tv maps film-series: %+v", em)
		}
	}
}

// TestTMDBMovieFilmSeriesTrace freezes the series container output for
// recorded movie documents: both films of one collection name the same series
// backdrop, and a film outside any collection (belongs_to_collection null)
// emits no series container.
func TestTMDBMovieFilmSeriesTrace(t *testing.T) {
	const image = "https://image.tmdb.org/t/p/original"
	spec := scraperByID(t, "tmdb-movie")
	vector := func(name, id, fixture, title, date string, containers []sdktest.TraceContainer) sdktest.ScraperTraceVector {
		return sdktest.ScraperTraceVector{
			Name:  name,
			Spec:  spec,
			Input: sdktest.TraceInput{ID: id, LookupDoc: loadFixture(t, fixture)},
			Want: sdktest.ScraperTrace{
				Requests: []sdktest.TraceRequest{{Role: "lookup", Method: "GET",
					URL:  "https://api.themoviedb.org/3/movie/" + id + "?append_to_response=credits,release_dates",
					Auth: sdktest.AuthPlacement{Location: "query", Name: "api_key", InURL: true}}},
				Fields: map[string]string{
					"title":                        title,
					"summary":                      "A synthetic movie document in the TMDB /movie/{id} shape.",
					"year":                         date[:4],
					"popularity":                   "12.5",
					"media.original-title":         title,
					"media.tagline":                "",
					"media.release-date":           date,
					"media.production-status":      "Released",
					"media.community-rating":       "7.1",
					"media.community-rating-votes": "840",
				},
				Artwork: map[string]string{
					"poster":   image + "/movie-" + id + "-poster.jpg",
					"backdrop": image + "/movie-" + id + "-backdrop.jpg",
				},
				Containers: containers,
			},
		}
	}
	saga := []sdktest.TraceContainer{{
		Kind:    "film-series",
		Artwork: map[string]string{"backdrop": image + "/saga-backdrop.jpg"},
	}}
	sdktest.RunScraperTraceConformance(t, []sdktest.ScraperTraceVector{
		vector("first film of a collection", "501", "tmdb-movie-saga-first.json", "Saga Origins", "1999-03-31", saga),
		vector("second film of the same collection", "502", "tmdb-movie-saga-second.json", "A Saga Returns", "2003-05-15", saga),
		vector("film outside any collection", "503", "tmdb-movie-standalone.json", "Standalone Feature", "2001-07-04", []sdktest.TraceContainer{}),
	})
}
