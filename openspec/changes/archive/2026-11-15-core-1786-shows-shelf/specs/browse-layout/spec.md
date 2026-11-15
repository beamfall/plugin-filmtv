## ADDED Requirements

### Requirement: Home layout leads with one card per show

The bundled home layout SHALL declare the show Entities as a shelf whose Source is `entity-items`
of kind `show`, placed before the `recently-added` shelf and after `next-up`, and SHALL NOT read
that Source from any other home section.

#### Scenario: TV Library first screen

- **WHEN** a client renders the home layout for a Library holding episodes of several shows
- **THEN** the first non-empty catalog shelf after Continue Watching and Next Up shows one card per
  show, not one card per episode
