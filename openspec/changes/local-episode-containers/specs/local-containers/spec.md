## ADDED Requirements

### Requirement: Local grouping for episode files

The plugin SHALL expose `LocalHydration(path)`, which reports for an episode file its series title,
season number, season title, episode index range and episode title, read from the path alone. A
season numbered 0 SHALL be titled `Specials`; any other season N SHALL be titled `Season N`. A path
that the parser does not classify as an episode SHALL be reported as not an episode.

#### Scenario: Dash-separated episode

- **WHEN** the path is `TV/Pioneer One/Season 01/Pioneer One - S01E02 - The Man From Orbit.mkv`
- **THEN** it is an episode of series `Pioneer One`, season 1 titled `Season 1`, index 2, episode
  title `The Man From Orbit`

#### Scenario: Specials

- **WHEN** the path is `Pioneer One - S00E01 - Pilot.mkv`
- **THEN** the season number is 0 and the season title is `Specials`

#### Scenario: Movie

- **WHEN** the path is `Night of the Living Dead (1968).mkv`
- **THEN** it is not an episode

### Requirement: Season relations

The season Entity Kind SHALL declare a `season-episode` relation to Media with cardinality one and
a `season-of` relation to the show Kind.

#### Scenario: Bundle declaration

- **WHEN** the bundle is loaded
- **THEN** the season Kind lists `season-episode` (to Media, cardinality one) and `season-of` (to
  `show`)

### Requirement: Local hydration marker

The bundle SHALL declare the flag Media field `filmtv-hydrated`, exported as `HydratedField`, for
Core's hydration seam to mark files whose local containers were written.

#### Scenario: Marker declared

- **WHEN** the bundle is loaded
- **THEN** its Media fields include `filmtv-hydrated` of type flag

### Requirement: Display title for episodes

The parser SHALL implement `MediaTitle(path, probe)`. For an episode file whose name carries an
episode title it SHALL return that episode title; otherwise it SHALL return the empty string, so
Core keeps the parsed title.

#### Scenario: Episode with a title

- **WHEN** the path is `Pioneer One - S01E01 - Earthfall.mkv`
- **THEN** `MediaTitle` returns `Earthfall` and the parsed title stays `Pioneer One`

#### Scenario: Episode without a title

- **WHEN** the path is `Pioneer.One.S01E01.1080p.mkv`
- **THEN** `MediaTitle` returns the empty string

### Requirement: Shows grid on the library home

The library home Layout SHALL include a grid of show Entities.

#### Scenario: Home layout

- **WHEN** the home Layout is read
- **THEN** it has a grid Section whose Source is `entity-items` over the `show` Kind
