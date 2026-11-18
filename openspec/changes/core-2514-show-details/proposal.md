# Show details, matched title, genres and cast from TMDB

Tickets: CORE-2514, CORE-2513 (`queue:core:main`).
Status: proposed.

## Why

The `tmdb-tv` show container map hydrates only the poster, overview and first air date, so a show page
has no backdrop, genres, network, status, last air date, content rating, community score or cast even
though the series document `/tv/{id}?append_to_response=credits,content_ratings` carries all of them
(CORE-2514). The show also keeps its folder spelling after a match ("A Mothers Son" rather than
"A Mother's Son"), because the map never supplies the TMDB title (CORE-2513).

## What Changes

- The `show` kind's `overview` field becomes `summary` (the field the detail hero renders as the
  overview), and the kind gains `last-air-date`, `network`, `certification-display`,
  `community-rating` and `community-rating-votes`. `status`, `first-air-date` and `certification` stay.
- The `tmdb-tv` show container map (plugin-sdk CORE-2514 fields) adds:
  - fields: summary, first and last air dates, status, primary network, US content rating, TMDB score
    and vote count;
  - a `backdrop` artwork slot next to the poster;
  - `TitlePath: name`, so a matched show takes the TMDB title and keeps the folder name only when
    the document has none;
  - `ContainerRelations` linking the show to the genres (`in-genre` ← `genre-of`) and cast
    (`credit` ← `credited-in`) the same lookup hydrates.
- A scraper reference-trace conformance vector freezes the show container output for a recorded
  series document, including an accented title.
- Requires the plugin-sdk release that adds `ArtworkMap`, `TitlePath` and `ContainerRelations` on
  container maps; the bundle's SDK floor and ceiling move to that release when it is tagged.
- Out of scope: episode-level data (CORE-0362) and the season map.

## Impact

- Code: `bundle.go`, its tests, a conformance vector and a recorded fixture.
- No change to the filename parser or the local mapper.
