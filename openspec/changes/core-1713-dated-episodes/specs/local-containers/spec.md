## MODIFIED Requirements

### Requirement: Local grouping for episode files

The plugin SHALL expose `LocalHydration(path)`, which reports for an episode file its series title,
season number, season title, episode index range and episode title, read from the path alone. A
season numbered 0 SHALL be titled `Specials`; any other season N SHALL be titled `Season N`. A
dated episode SHALL be reported in the season numbered by its air year, with its month and day
(`MMDD`) as its episode index, and its air date. A path that the parser does not classify as an
episode SHALL be reported as not an episode.

#### Scenario: Dash-separated episode

- **WHEN** the path is `TV/Pioneer One/Season 01/Pioneer One - S01E02 - The Man From Orbit.mkv`
- **THEN** it is an episode of series `Pioneer One`, season 1 titled `Season 1`, index 2, episode
  title `The Man From Orbit`

#### Scenario: Specials

- **WHEN** the path is `Pioneer One - S00E01 - Pilot.mkv`
- **THEN** the season number is 0 and the season title is `Specials`

#### Scenario: Dated episode

- **WHEN** the path is `Synthetic Daily 2023-03-01.mkv`
- **THEN** it is an episode of series `Synthetic Daily`, season 2023 titled `Season 2023`, index 301,
  air date `2023-03-01`

#### Scenario: Movie

- **WHEN** the path is `Night of the Living Dead (1968).mkv`
- **THEN** it is not an episode

### Requirement: Display title for episodes

The parser SHALL implement `MediaTitle(path, probe)`. For an episode file it SHALL return the
episode code, followed by ` · ` and the episode title when the name carries one. The code SHALL be
the ISO air date for a dated episode, otherwise `S` and the two-digit season, `E` and the two-digit
episode, and for a range `-` and the two-digit last episode. For anything else it SHALL return the
empty string, so Core keeps the parsed title. Because the code is zero-padded and the date is ISO,
ordering a series' episodes by display title orders them by season and episode (up to episode 99)
or by air date.

#### Scenario: Episode with a title

- **WHEN** the path is `Pioneer One - S01E01 - Earthfall.mkv`
- **THEN** `MediaTitle` returns `S01E01 · Earthfall` and the parsed title stays `Pioneer One`

#### Scenario: Episode without a title

- **WHEN** the path is `Pioneer.One.S01E01.1080p.mkv`
- **THEN** `MediaTitle` returns `S01E01`

#### Scenario: Multi-episode file

- **WHEN** the path is `Synthetic Show S01E03-E04.mkv`
- **THEN** `MediaTitle` returns `S01E03-04`

#### Scenario: Daily files

- **WHEN** the paths are `Synthetic Daily 2023-03-02.mkv` and `Synthetic Daily 2023-03-01.mkv`
- **THEN** `MediaTitle` returns `2023-03-02` and `2023-03-01`, which order by air date

#### Scenario: Movie

- **WHEN** the path is `Synthetic Film (2010).mkv`
- **THEN** `MediaTitle` returns the empty string
