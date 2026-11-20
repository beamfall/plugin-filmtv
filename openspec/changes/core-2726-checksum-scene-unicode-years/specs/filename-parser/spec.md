## MODIFIED Requirements

### Requirement: A scene tag before a checksum is release noise

After a leading parenthesized year, a scene tag (`PROPER`, `REPACK`, `RERIP`) that only checksum
brackets and release tags follow SHALL be release noise. When a bracketed title stands before the
year, that title stays the title. When nothing stands before the year, the name SHALL have no title
and SHALL NOT parse. A release word that can be a title SHALL keep the scene tag as the title, as
SHALL other release words in that place.

#### Scenario: Scene tag

- **WHEN** a file is `/movies/[REC] (2007) PROPER.[DEADBEEF].mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Scene tag, checksum and release tag

- **WHEN** a file is `/movies/[REC] (2007) PROPER.[DEADBEEF].1080p.mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Scene tag after a bare leading year

- **WHEN** a file is `/movies/(2007) PROPER.[DEADBEEF].mkv`
- **THEN** it does not parse

#### Scenario: Scene tag before a title-capable word

- **WHEN** a file is `/movies/[Group] (2013) Proper Uncut [DEADBEEF].mkv`
- **THEN** it parses to title `Proper Uncut`, year 2013

#### Scenario: Ordinary release word

- **WHEN** a file is `/movies/[Group] (2013) DV [DEADBEEF].mkv`
- **THEN** it parses to title `DV`, year 2013

## ADDED Requirements

### Requirement: A checksum folder names no series

A series name taken from a show folder SHALL NOT include a CRC32 checksum bracket. A folder that
holds nothing else SHALL name no series.

#### Scenario: Checksum show folder

- **WHEN** a file is `/tv/[DEADBEEF]/Season 01/S01E01.mkv`
- **THEN** it does not parse

### Requirement: Any script's letters bound a year

A four-digit number SHALL NOT be read as a year when a letter, digit or combining mark of any
script directly precedes or follows it. This applies both to the release year and to the bare year
that ends a series name before an absolute number.

#### Scenario: Year after an accented letter

- **WHEN** a file is `/tv/Show (2011) Café1999 0001.mkv`
- **THEN** it parses to title `Show`, year 2011, no episode

#### Scenario: Accented series name with digits

- **WHEN** a file is `/tv/Café1999 0001.mkv`
- **THEN** it parses to series `Café1999`, no year, absolute episode 1
