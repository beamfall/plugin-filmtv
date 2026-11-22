# TV child listings for media assignment

Ticket CORE-3129 (`queue:core:main`).
Decision ADR-0068 amendment H, Declared child listings.

## Why

Operators assigning an episode need to see a chosen show's seasons and episodes with their
provider ordinals.

## What changes

- Declare season and episode listings for TMDB TV, TheTVDB, and TVmaze.
- Preserve season zero when a provider supplies it. AniList remains show-level.
- Filter TheTVDB's extended series record to aired-order seasons.
- Enable free-text search for TVmaze and read its search and child listings from root arrays.
  TVmaze specials without an episode number keep their airing season and leave the index empty.

## Verification and rollback

Validate the bundle and exercise each enabled level against trimmed provider-shaped fixtures.
Reverting the declaration commit restores the previous scraper behavior.
