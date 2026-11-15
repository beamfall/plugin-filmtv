# browse-layout Specification

## Purpose
What the Film & TV plugin's browse layout declares to Core: the filter bar options it offers and
where their values come from, so no fixed network or service list ships in the bundle.

## Requirements

### Requirement: Browse filter bar declares no fixed network options

The bundled home layout's filter-bar sections SHALL declare options only on the closed `class`
vocabulary and SHALL NOT declare fixed network or studio values.

#### Scenario: Library without a given network

- **WHEN** a client renders the browse filter bar for a Library
- **THEN** it offers only the Movies and Episodes chips, and no chip can narrow the grid to a
  network the Library does not hold

### Requirement: Home layout leads with one card per show

The bundled home layout SHALL declare the show Entities as a shelf whose Source is `entity-items`
of kind `show`, placed before the `recently-added` shelf and after `next-up`, and SHALL NOT read
that Source from any other home section.

#### Scenario: TV Library first screen

- **WHEN** a client renders the home layout for a Library holding episodes of several shows
- **THEN** the first non-empty catalog shelf after Continue Watching and Next Up shows one card per
  show, not one card per episode
