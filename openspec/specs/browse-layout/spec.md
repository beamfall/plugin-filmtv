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
