# TV episode parsing, series matching and class-neutral home rows

Tickets: CORE-2311 (T07), CORE-2313 (T09), CORE-2320 (T16) (`queue:core:main`).
Status: proposed.

## Why

A tower QA pass on a real TV library found episodes filed as movies with raw file names as titles
(`1e01-Yesterday's Jam`, `3 01 From Hell`, `Dealbreakers Talk Show 0001`), every matched episode of
a show titled with the show name and dated with the show's premiere year, scene names without
apostrophes (`A Mothers Son`) failing to match the provider's `A Mother's Son`, TheTVDB shown as a
required key although TV matches through TMDB, and a Films library showing rows titled
"New Movies & Episodes" and "All Movies & Episodes".

## What Changes

- The filename parser reads three more episode forms:
  - a season-by-episode code with a bare `e` (`1e01-Yesterday's Jam`), anywhere a season/episode
    token may stand;
  - a leading `season episode` number pair (`3 01 From Hell`), only inside a season folder whose
    number, when it has one, agrees;
  - a trailing absolute episode number zero-padded to four digits (`Dealbreakers Talk Show 0001`),
    or to three digits after ` - ` (`Naruto - 001`), when the name carries no other episode token
    and no year. It carries an index but no season; the local
    grouping files it under Season 1 and its display code is `E01`.
- The match scorer compares titles with apostrophes removed instead of turned into word breaks, so
  `A Mothers Son` and `A Mother's Son` are the same title.
- `tvdb-series` declares its credential optional: TV matches through `tmdb-tv` and the built-in TMDB
  key, and identification skips TheTVDB while its PIN is unset.
- The home layout's `recently-added` and catch-all grid titles name no content class: "Recently
  Added" and "All Titles", so a Films or TV library is not labelled "Movies & Episodes".

## Not in this change (needs SDK and Core)

- A matched episode titled with the series name and dated with its premiere year: identification
  writes the series lookup's `title` and `year` onto the episode, and Core counts a Media row as
  matched only when its title was written (`title_identified`). Dropping those mappings here would
  report every matched episode as unmatched. Core has to keep the parser's display title for a
  series-level match (the `core-1713-identify-keeps-parser-title` companion named by CORE-1713), and
  a per-episode lookup has to supply the episode's own air date.

- A per-episode lookup (`/tv/{id}/season/{s}/episode/{e}`): the scrape engine substitutes no season
  or episode placeholder, so it cannot be declared.
- Show and season hydration from the series lookup: an `EntityMap` needs a non-empty `Path`
  resolving to an element, so the lookup's root document cannot hydrate one show Entity, and the
  show Kind has no Media-targeted relation.
- The order of the `recently-added` Source and the title of a detail page's related row are Core
  behaviour: `SourceSpec` and `SectionSpec` declare no sort or related-row title.

## Impact

Core picks this up when it pins the plugin-filmtv release carrying it. No contract field changes.
