# Lead the home layout with a Shows shelf

Ticket: CORE-1786 (`queue:core:main`), acceptance criterion 1: on the synthetic corpus the TV
library's primary shelf shows one card per show.

## Why

The first shelf a TV Library shows today is "New Movies & Episodes": one card per episode file,
with no show context. The show Entities sit in a grid below it. People browse by the show they
follow, so the first row should be the shows.

## What Changes

- The `browse-shows` section becomes a shelf and moves to directly after Next Up, ahead of
  "New Movies & Episodes". Its Source (`entity-items`, kind `show`) is unchanged and no other home
  section reads it, so no title repeats. Its See All page keeps the full show list.
- The Movies and Episodes grids, the filter bar and the catch-all grid keep their places.

## Impact

- `bundle.go` (the home layout) and `bundle_test.go`. No new vocabulary: `shelf` and
  `entity-items` are existing SDK kinds. Core picks it up by repinning this module.
