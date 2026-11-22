# The home layout is recommendations only

Ticket: CORE-3066 (`queue:core:main`)

## Why

The owner asked for the Episodes and All Titles sections to come off a Library's Recommended tab
(2026-11-21). Core renders this plugin's home layout as that tab, and the layout ended with an
Episodes grid, a Browse filter bar and an All Titles grid: flat lists of every Media, which repeat
the Library tab's Media view under a page of shelves.

## What changes

- The home layout drops the `browse-episodes` grid, the `browse-filter` filter bar and the
  `browse-all` grid. The filter bar goes with them because it narrows the grid below it.
- Every Media, identified or not, stays reachable through Core's Library tab Media view, so
  removing the grids hides nothing. This replaces the All Titles reach the CORE-2535 change named.
- README and tests follow: a home test pins no grid, filter bar, class filter or unnarrowed
  `all-items` Source.

## Non-goals

- No change to the shelves, detail layouts, Media fields or class vocabulary.
- Core needs a repin to the tagged release before the Recommended tab changes.

## Failure modes

- A client on an older Core still renders the plugin's declared sections, so it shows the shorter
  home as soon as it reads the new manifest; nothing depends on the removed section IDs.
