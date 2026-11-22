# TV child listings for media assignment

Ticket CORE-3129 (`queue:core:main`).
Decision ADR-0068 amendment H, Declared child listings.

## Why

Operators assigning an episode need to see a chosen show's seasons and episodes with their
provider ordinals.

## What changes

- Declare season and episode listings for TMDB TV, TheTVDB, and TVmaze wherever the pinned SDK can
  resolve the provider's response shape and ordering.
- Preserve season zero when a provider supplies it. AniList remains show-level.
- TheTVDB needs an array of aired-order seasons. Its extended series record mixes orders, and the
  current path predicate selects one row. TVmaze needs support for root arrays in search and child
  listing responses. Its unnumbered specials also need a way to map to season zero.

## Verification and rollback

Validate the bundle and exercise each enabled level against trimmed provider-shaped fixtures.
Reverting the declaration commit restores the previous scraper behavior.
