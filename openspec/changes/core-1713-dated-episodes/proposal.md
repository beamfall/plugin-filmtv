# Date-named episodes, episode ranges and SxxEyy on episode cards

Ticket: CORE-1713 (`queue:core:main`).
Status: proposed.

## Why

A daily show names its files by air date (`Synthetic Daily 2023-03-01.mkv`). The parser reads such a
name as a movie titled with the series name and a year, so two days of one show are
indistinguishable in the library. A multi-episode file with no episode title
(`Synthetic Show S01E03-E04.mkv`) is shown with the series name, and the library landing lists
episode cards with no season or episode numbers at all.

CORE-0362 / CORE-1148 already parse multi-episode chains into `Index` / `IndexEnd`; this change
reuses that parse and only adds the date form and the display title.

## What Changes

- The filename parser recognises an air date (`YYYY-MM-DD`, also with `.`, `_` or space
  separators, calendar-checked) as the episode token when the name carries no season/episode
  token. Such a file is an `episode`: the text before the date is the series title, the text after
  it the `episode-title`, and no season or episode numbers are invented.
- The parser emits the air date as the typed `air-date` Media field (the bundle already declares
  it as a sortable date) through `ParseTypedMediaFields`, alongside `episode-title`.
- `MediaTitle` (Core's per-file display-title hook) now returns an episode code for every episode:
  the air date (`2023-03-01`) for a dated episode, otherwise `S01E03`, or `S01E03-04` for a range,
  followed by ` · <episode title>` when the name carries one. The title is what every card and the
  landing grid show, and because the code is zero-padded and the date is ISO, ordering by title
  orders a show's episodes by season and episode, or by air date.
- `LocalHydration` groups a dated episode under the season of its air year (`Season 2023`, number
  2023, Plex's convention for dated shows) ordered by month and day, and reports its `AirDate`.
- The `tmdb-tv` and `tvmaze-show` scrapers stop mapping the series premiere date onto
  `media.air-date`. Identification looks up the series, so that mapping wrote the premiere over
  every episode's air date, including the date parsed from a date-named episode.

## Impact

Core already has the `MediaTitler` hook and the typed field parser seam. Identification wrote the
series name over the episode title; a companion core change (`core-1713-identify-keeps-parser-title`)
makes identification keep a parser-owned display title. Core picks this behaviour up when it pins a
plugin-filmtv release carrying this change (a core scan test proving it end to end lands with that
repin). When CORE-1226's card subtitle lands, the episode code can move
from the title to the subtitle.
