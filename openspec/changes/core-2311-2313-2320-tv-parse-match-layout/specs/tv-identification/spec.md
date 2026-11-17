## ADDED Requirements

### Requirement: TheTVDB is optional

The `tvdb-series` scraper SHALL declare its credential optional, so a library with only the built-in
TMDB key matches TV through `tmdb-tv` and TheTVDB is skipped while its PIN is unset.

#### Scenario: No TheTVDB PIN

- **WHEN** identification runs with the TMDB key set and no TheTVDB PIN
- **THEN** `tvdb-series` is skipped and `tmdb-tv` identifies the episode

### Requirement: Apostrophes do not split title words

The match scorer SHALL remove apostrophes when it normalises titles, so a scene name that drops them
compares equal to the provider title.

#### Scenario: Scene name without an apostrophe

- **WHEN** the parsed title is `A Mothers Son` and the candidate title is `A Mother's Son`
- **THEN** the titles score as an exact match
