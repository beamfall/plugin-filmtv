# Film series from TMDB collections

Tickets: CORE-3152 (`queue:core:main`), split from CORE-0363.
Status: proposed.

## Why

The bundle declares a `film-series` container kind, but nothing creates one. TMDB's movie document
carries `belongs_to_collection` {id, name, poster_path, backdrop_path} on every franchise film, so the
series can be built automatically when a film is identified.

## What Changes

- `film-series` declares a single-holder relation `film-series-film` to Media, so a film belongs to
  at most one series and core treats the series as the film's container.
- The `tmdb-movie` scraper gains two maps over `belongs_to_collection`:
  - a non-container map that creates (or reuses) the `film-series` Entity named after the collection,
    with its `tmdb-id` and poster, and links it to the film;
  - a container map that adds the collection backdrop to that series.
- A film with no `belongs_to_collection` gets no series. A re-identified film that left its
  collection loses the link, and one that moved joins the new series (core CORE-3152 seam).
- Members are ordered by release year through the core seam. Day precision needs an SDK ordinal from
  the document root and is out of scope.
- CORE-0363's other parts (ratings, certification, trailers, language, rate limits) stay there.

## Impact

- Code: `bundle.go`, its tests, a scraper trace vector and recorded fixtures.
- Requires core with the CORE-3152 seam (`RemoveMediaRelations`, year ordinal for single-holder
  links). Without it, links are written but never cleared and members sort by title.
- The series identity is name-keyed in core (the Entity ID hashes library, kind and name), so two
  collections with the same name in one library share a series.
