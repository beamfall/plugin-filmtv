# Show and season hydration and per-episode lookups from TMDB

Ticket: CORE-2335 (`queue:core:main`).
Status: proposed.

## Why

On a running server, show and season pages carry no poster, overview or year: only episodes get art.
The `tmdb-tv` scraper maps the series document onto each episode and hydrates genres, studios and
people, but never the show and season Entities the local mapper builds from the folder layout. Every
episode also takes the series overview, first-air year and (unresolvable) still from the series
document instead of its own.

## What Changes

- The `show` and `season` kinds declare an `overview` text field.
- `tmdb-tv` declares two container entity maps (plugin-sdk CORE-2335): the series document root
  hydrates the `show` holding the episode (poster, overview, first air date), and
  `seasons[season_number={sequence}]` hydrates its `season` (poster, overview, air date).
- `tmdb-tv` declares the per-episode lookup `/tv/{id}/season/{sequence}/episode/{index}`: the
  episode's own overview becomes its summary, its air date its year, and its still its thumb. The
  episode keeps the parser's title.
- Requires the plugin-sdk release that adds container maps and item lookups; the bundle's SDK floor
  and ceiling move to that release when it is tagged.

## Impact

- Code: `bundle.go` and its tests.
- No change to the filename parser or the local mapper.
