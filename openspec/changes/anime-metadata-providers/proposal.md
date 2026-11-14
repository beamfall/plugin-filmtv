# Add TVmaze and AniList metadata providers and an absolute-episode mapper

Ticket: CORE-1699 (`queue:core:main`). Status: proposed.

## Why

The plugin identifies films and series through TMDB and TheTVDB only. Anime libraries usually rely
on an anime-specific catalogue, and TVmaze is a free, key-less TV catalogue that needs no account.
Anime releases are also often numbered by absolute episode (`Show - 13`) rather than by season and
episode, and nothing in the plugin can turn an absolute number into a season and episode.

## What Changes

- A `tvmaze-show` scraper. TVmaze needs no key. Its data is licensed CC BY-SA 4.0, so the scraper
  declares an attribution that credits TVmaze and links to it. Requests are spaced 500 ms apart,
  inside TVmaze's limit of at least 20 calls per 10 seconds per address. It looks a show up by its
  TVmaze id, and declares provider-ID lookups for TheTVDB and IMDb ids.
- Two AniList scrapers, `anilist-series` (implies class `episode`) and `anilist-movie` (implies
  class `movie`), using AniList's GraphQL API. Each operator supplies their own AniList access
  token (`anilist-access-token`). The credential is optional: without it the scrapers are left out
  and send no requests. Requests are spaced 2 s apart, inside AniList's current limit of 30 per
  minute.
- A plugin-owned mapper from an absolute episode number to a season and episode, over a series'
  regular episodes in season order, with a decoder for TVmaze's embedded episode list.

## Non-goals

- AniDB. Its HTTP API returns XML, which no scraper kind reads; it requires a client registered by
  the project; and its daily request caps and ban-on-violation policy need a persistent cache that
  the scrape engine does not have. The reasons are recorded on CORE-1699.
- Free-text search on TVmaze. TVmaze's search response is a bare JSON array. The plugin SDK refuses
  a search scraper without a candidates path, although the Core engine already accepts the whole
  response as the candidate array. Search is added once the SDK accepts it.
- Summaries from TVmaze and AniList. Both return HTML markup in their descriptions, and the
  transform vocabulary has no HTML-to-text transform, so the field is left unmapped rather than
  shown with tags.
- Reading the absolute number from a filename, the season-agnostic scorer fallback and
  episode-level lookups. They belong to CORE-0362, which can call the mapper.

## Failure modes

- AniList allows commercial use free of charge only below a revenue threshold, and its terms bar
  use in a competing anime list or tracking service. An operator's own token keeps each operator
  the API user; a commercial distribution decision stays with the owner.
- TVmaze's CC BY-SA 4.0 licence requires credit and share-alike for the data. The attribution
  covers the credit; share-alike applies to anyone redistributing the data.
- An absolute number past the last regular episode, or a list with no regular episodes, maps to
  nothing rather than to a guess.

## Acceptance evidence

`go test ./...` (bundle validity, attribution, optional credential, request spacing, scraper trace
vectors over recorded documents, mapper vectors over a recorded TVmaze episode list) and
`script/gate.sh`.

## Rollback

Revert the code commit. Media identified through the new scrapers keep their plain fields; a bundle
without the scrapers no longer offers them for identification.
