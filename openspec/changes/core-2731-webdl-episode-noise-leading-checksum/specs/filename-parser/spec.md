## ADDED Requirements

### Requirement: A hyphen-split release tag is not a release group

A release tag split by a hyphen (`WEB-DL`) SHALL be one release tag in scene-tag noise, not a tag
followed by a hyphenated release group.

#### Scenario: Scene tags before WEB-DL

- **WHEN** a file is `/movies/[REC] (2007) PROPER.REPACK.WEB-DL.mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Release group after WEB-DL

- **WHEN** a file is `/movies/(2007) PROPER.WEB-DL-GRP.mkv`
- **THEN** it does not parse

### Requirement: Episode scene noise names no series

When only scene-tag noise stands between a bare leading parenthesized year and an episode marker,
the episode SHALL have no series name of its own and SHALL take it from the show folder.

#### Scenario: Scene noise before an episode marker

- **WHEN** a file is `/tv/Fringe/Season 01/(2007) PROPER.1080p.S01E01 - Pilot.mkv`
- **THEN** it parses to title `Fringe`, year 2007, class `episode`, season 1, episode 1

### Requirement: A leading checksum is not a release group

A checksum bracket at the start of a name SHALL NOT be a release-group bracket: a year-named title
after it SHALL be the title, not the year, and the checksum SHALL NOT end the title.

#### Scenario: Leading checksum before a year-named title

- **WHEN** a file is `/movies/[DEADBEEF] 1923.mkv`
- **THEN** it parses to title `1923`, no year

#### Scenario: Leading release group before a bare year

- **WHEN** a file is `/movies/[Group] 1923.mkv`
- **THEN** it parses to title `[Group]`, year 1923

### Requirement: A checksum keeps the word before it in the title

A checksum after the title SHALL end the title the way a year does: a release word directly before
it (`Uncut`) SHALL stay title text, and SHALL NOT be stripped as a trailing edition word.

#### Scenario: Release word before a checksum

- **WHEN** a file is `/movies/Movie Uncut [DEADBEEF].mkv`
- **THEN** it parses to title `Movie Uncut`

#### Scenario: Release word at the end

- **WHEN** a file is `/movies/Movie Uncut.mkv`
- **THEN** it parses to title `Movie`
