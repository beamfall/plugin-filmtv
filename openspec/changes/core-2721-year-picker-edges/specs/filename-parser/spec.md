## ADDED Requirements

### Requirement: An underscore bounds a year

The filename parser SHALL read a year bounded by underscores as it reads one bounded by spaces or
dots, for the release year, the year-run and leading-year rules, and series names.

#### Scenario: Underscored film year

- **WHEN** a file is `/movies/Heat_1995_1080p.mkv`
- **THEN** it parses to title `Heat`, year 1995

#### Scenario: Underscored year after a title year

- **WHEN** a file is `/movies/2012_2009_1080p.mkv`
- **THEN** it parses to title `2012`, year 2009

### Requirement: A series year before an absolute number stays in the series name

When a bare year directly precedes a four-digit absolute episode number and the file has the
episode context that number needs, the parser SHALL read the year as part of the series name, not
as the release year. Without that context the name SHALL parse as before.

#### Scenario: Spaced series year

- **WHEN** a file is `/tv/Space 1999/Season 01/Space 1999 0001.mkv`
- **THEN** it parses to series `Space 1999`, no year, absolute episode 1

#### Scenario: No context

- **WHEN** a file is `/incoming/Space 1999 0001.mkv`
- **THEN** it parses to the film `Space`, year 1999

### Requirement: Release-group brackets and checksums are never film title text

Leading release-group brackets MAY be separated by spaces, dots, underscores or hyphens, and years
inside any of them SHALL NOT count. A film title SHALL drop its leading release-group brackets when
only a year-named title follows them, and SHALL keep them otherwise (`[REC] 2`). A CRC32 checksum
bracket SHALL NOT be title text.

#### Scenario: Separated groups

- **WHEN** a file is `/movies/[Fansub].[Group (1999)] (2000) Title.mkv`
- **THEN** it parses to title `Title`, year 2000

#### Scenario: Group before a year-named title

- **WHEN** a file is `/movies/[Group] 2012 (2009).mkv`
- **THEN** it parses to title `2012`, year 2009

#### Scenario: Only checksums

- **WHEN** a file is `/movies/[CAFEBABE] (2007) [DEADBEEF].mkv`
- **THEN** it does not parse

### Requirement: A scene tag before a checksum is release noise

After a leading parenthesized year, a scene tag (`PROPER`, `REPACK`, `RERIP`) followed only by
checksum brackets SHALL be release noise, so the bracketed title before the year stays the title.
Other release words in that place SHALL stay the title.

#### Scenario: Scene tag

- **WHEN** a file is `/movies/[REC] (2007) PROPER.[DEADBEEF].mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Ordinary release word

- **WHEN** a file is `/movies/[Group] (2013) DV [DEADBEEF].mkv`
- **THEN** it parses to title `DV`, year 2013

### Requirement: A dashed episode can follow a dashed year

When a dashed number in the year range (` - 2011`) is followed by another dashed number, the parser
SHALL read the year as the series' year and the later number as the absolute episode, given the
context a dashed number after a year needs. Underscore-spaced dashes (`_-_`) SHALL read the same.

#### Scenario: Dashed year then episode

- **WHEN** a file is `/tv/Show - 2011 - 12.mkv`
- **THEN** it parses to series `Show`, year 2011, absolute episode 12

#### Scenario: Underscore-spaced dashes

- **WHEN** a file is `/tv/Show_-_2011_-_12.mkv`
- **THEN** it parses to series `Show`, year 2011, absolute episode 12

#### Scenario: Dashed year alone

- **WHEN** a file is `/tv/Show - 2015.mkv`
- **THEN** it parses to title `Show`, year 2015, no episode
