# TMDB ratings on Media

Ticket: CORE-2399 (core `queue:core:main`). Core change: `core-2399-media-ratings` in beamfall.

## Why

The plugin declares `certification` and `certification-display` but never maps them, and maps no
community score, though the TMDB documents it requests carry both. Clients cannot show a rating.

## What changes

- Declare number Media fields `community-rating` (TMDB `vote_average`, 0 to 10, label "TMDB
  rating") and `community-rating-votes` (`vote_count`).
- `tmdb-movie` requests `append_to_response=credits,release_dates` and maps the US theatrical
  certification (else digital, else the first US release's) onto `certification` and
  `certification-display`.
- `tmdb-tv` requests `append_to_response=credits,content_ratings` and maps the US TV rating onto
  `certification-display` only; TV ratings are outside the MPAA scale that orders Profile ceilings.
- Series and episode documents map score and votes; the episode's own values win for an episode.

## Non-goals

No IMDb or Rotten Tomatoes scores (no licensed keyless source; scraping forbidden). No new requests:
the added data rides on the existing lookup.

## Failure modes

A title with no US certification maps none and stays unrated. A non-scale movie certification
("NR") is stored as given and is blocked under a ceiling, as any unordered value is today.

## Acceptance evidence

`TestTMDBRatingMappings` and `TestProviderIDLookupTemplates`; `go test ./...`; golangci-lint.

## Rollback

Revert the commit; stored values stay until the next refresh.
