## ADDED Requirements

### Requirement: Home row titles name no content class

The bundled home layout serves both film and TV libraries, so the titles of its `recently-added`
shelf and its unfiltered catch-all grid SHALL NOT name a content class.

#### Scenario: Films library

- **WHEN** a client renders the home layout for a library holding only films
- **THEN** the new-items shelf reads "Recently Added" and the catch-all grid "All Titles", not
  "Movies & Episodes"
