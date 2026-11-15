# Drop the fixed network chips from the browse filter bar

Ticket: CORE-1711 (`queue:core:main`).
Status: proposed.

## Why

The home layout's `browse-filter` section declared two fixed `network` filter options naming
specific broadcasters. Picking one on a Library that holds no Media from that network narrows the
adjacent grid to nothing. A filter-bar option in the plugin SDK and Core's manifest is a fixed
field, value and label; neither has a source that derives options from values present in the
Library.

## What Changes

- The `browse-filter` section keeps its `class` options (Movies, Episodes) and declares no network
  or studio options.
- The `network` Media field and the tmdb-tv `media.network` mapping are unchanged, so the data is
  still hydrated and filterable.
- Network chips can return once Core can derive filter-bar options from Library values; that needs
  a Core and SDK change outside this repo.

## Impact

Clients render two chips instead of four in the browse filter bar. No API, SDK or wire change.
