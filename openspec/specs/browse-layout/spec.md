# browse-layout Specification

## Purpose
What the Film & TV plugin's browse layout declares to Core: the filter bar options it offers and
where their values come from, so no fixed network or service list ships in the bundle.

## Requirements

### Requirement: Browse filter bar declares no fixed network options

The bundled layouts' filter-bar sections, where any are declared, SHALL declare options only on the
closed `class` vocabulary and SHALL NOT declare fixed network or studio values.

#### Scenario: Library without a given network

- **WHEN** a bundled layout declares a filter bar and a client renders it for a Library
- **THEN** it offers only `class` chips, and no chip can narrow a section to a network the Library
  does not hold

### Requirement: Home layout leads with one card per show

The bundled home layout SHALL declare the show Entities as a shelf whose Source is `entity-items`
of kind `show`, placed before the `recently-added` shelf and after `next-up`, and SHALL NOT read
that Source from any other home section.

#### Scenario: TV Library first screen

- **WHEN** a client renders the home layout for a Library holding episodes of several shows
- **THEN** the first non-empty catalog shelf after Continue Watching and Next Up shows one card per
  show, not one card per episode

### Requirement: Home layout declares recommendations only

The bundled home layout SHALL NOT declare a grid or filter-bar section, a section narrowed by the
`class` field, or a section whose Source is `all-items` without an Entity filter. Every Media of a
Library SHALL stay reachable through the host's Library browse, not through the home layout.

#### Scenario: TV Library Recommended tab

- **WHEN** a client renders the home layout for a TV Library
- **THEN** it shows the hero and shelves and no Episodes, Browse or All Titles section

#### Scenario: Unidentified file

- **WHEN** a Library holds a file with no class field
- **THEN** the home layout does not list it in a catch-all grid, and the Library tab's Media view
  still lists it
