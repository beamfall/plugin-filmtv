# media-subtitle Specification

## Purpose
Declare the subtitle template hosts render under each Film and TV Media title, so episodes that share a show title stay distinguishable.

## Requirements

### Requirement: Film and TV Media declare a distinguishing subtitle

The bundle SHALL declare `MediaSubtitle` segments `S{sequence}`, `E{index}` and
`{episode-title}`, so a host renders an episode as `S<season> · E<episode> · <title>` and drops any
segment whose value is missing.

#### Scenario: Parsed episode

- **WHEN** an episode has sequence 5, index 3 and episode title "Reunion"
- **THEN** the rendered subtitle is `S5 · E3 · Reunion`

#### Scenario: Movie

- **WHEN** a movie has no sequence, index or episode title
- **THEN** the rendered subtitle is empty

### Requirement: The episode lookup supplies the episode title

The `tmdb-tv` per-episode item lookup SHALL map the episode document's `name` onto the
`episode-title` Media field.

#### Scenario: Filename without a title

- **WHEN** an identified episode's filename carries no episode title
- **THEN** the item lookup writes the episode's name to `episode-title`
