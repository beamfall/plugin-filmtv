## ADDED Requirements

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

## MODIFIED Requirements

### Requirement: Browse filter bar declares no fixed network options

The bundled layouts' filter-bar sections, where any are declared, SHALL declare options only on the
closed `class` vocabulary and SHALL NOT declare fixed network or studio values.

#### Scenario: Library without a given network

- **WHEN** a bundled layout declares a filter bar and a client renders it for a Library
- **THEN** it offers only `class` chips, and no chip can narrow a section to a network the Library
  does not hold
