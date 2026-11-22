## ADDED Requirements

### Requirement: TV child listings expose provider ordinals

The plugin SHALL declare season and episode listings for TMDB TV, TheTVDB, and TVmaze using their
existing scraper endpoints and credentials. AniList SHALL remain show-level.

#### Scenario: Pick a season and an episode

- **WHEN** an operator opens the child listings of a TV search candidate and selects a season
- **THEN** the season list carries provider keys, labels, season ordinals, and available years and artwork
- **AND** the episode list carries keys, labels, season ordinals, and available episode ordinals, years, and artwork for that season

#### Scenario: Pick a specials season

- **WHEN** TMDB TV or TheTVDB returns a specials season numbered zero
- **THEN** the season list includes it with sequence zero

#### Scenario: TheTVDB provides multiple season orders

- **WHEN** TheTVDB provides aired and alternate season orders
- **THEN** the season list contains only aired-order seasons

#### Scenario: Pick an unnumbered TVmaze special

- **WHEN** TVmaze returns a special with no episode number
- **THEN** the episode list includes it with its airing season ordinal and an empty episode ordinal
