## ADDED Requirements

### Requirement: Shared home has no film-only rail

The Film & TV browse layout SHALL NOT declare a fixed movie-class grid or shelf. Loose Media SHALL remain reachable through the host's Library browse (All Titles until CORE-3066); removing a rail SHALL NOT remove or reclassify Media or bypass the owner Unmatched / fix-match workflow.

#### Scenario: Unmatched file classified as a film in a TV Library

- **WHEN** the shared home layout renders a TV Library with a movie-class file
- **THEN** no Movies rail promotes that file
- **AND** the file remains available for owner matching and deliberate catalog browsing
