## ADDED Requirements

### Requirement: A leading standalone year is title text

The filename parser SHALL NOT read a standalone year as the release year when no title text comes
before it, or when only separators (spaces, dots, hyphens) stand between it and a later year. That year SHALL stay in the title, and the next qualifying year, if any, SHALL be the release
year. Years inside the leading release-group brackets SHALL NOT count.

#### Scenario: Two leading years

- **WHEN** a file is `/movies/2012.2009.1080p.BluRay.x264.mkv`
- **THEN** it parses to title `2012`, year 2009, class `movie`

#### Scenario: Year-named title with a later year

- **WHEN** a file is `/movies/2019 After The Fall Of New York 1983 REMASTERED BDRip x264-GHOULS.mkv`
- **THEN** it parses to title `2019 After The Fall Of New York`, year 1983

#### Scenario: Number at the end of the title

- **WHEN** a file is `/movies/Blade.Runner.2049.2017.1080p.mkv`
- **THEN** it parses to title `Blade Runner 2049`, year 2017

#### Scenario: Lone leading year

- **WHEN** a file is `/movies/2012.mkv`
- **THEN** it parses to title `2012`, no year

#### Scenario: Series named after a year

- **WHEN** a file is `/tv/1923/Season 01/1923.S01E01.mkv`
- **THEN** it parses to title `1923`, class `episode`, season 1, episode 1

### Requirement: A tight rising year range starts with the year

When two years are joined by a single hyphen and the second is later (`2008-2013`), the filename
parser SHALL read the first as the release year and end the title before it. The second year SHALL
NOT be read as the release year.

#### Scenario: Year range

- **WHEN** a file is `/movies/Fringe 2008-2013.mkv`
- **THEN** it parses to title `Fringe`, year 2008

#### Scenario: Range after a bracketed title

- **WHEN** a file is `/movies/[REC] 2007-2009.mkv`
- **THEN** it parses to title `[REC]`, year 2007

### Requirement: A year after only release-group brackets is the year

When no standalone year in a film name has title text before it, the filename parser SHALL read the
first one that only release-group brackets precede as the release year, with those brackets as the
title. An episode name drops its leading brackets, so this SHALL NOT apply to it.

#### Scenario: Bracketed title before a bare year

- **WHEN** a file is `/movies/[REC].2007.mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Bracketed episode name keeps its year-named series

- **WHEN** a file is `/incoming/[Group] 1923.S01E01.mkv`
- **THEN** it parses to title `1923`, no year, class `episode`, season 1, episode 1

### Requirement: A bare-year series name defers to a matching show folder

When an episode's whole series name is a year and the show folder carries that same year, the
filename parser SHALL take the series name and year from the show folder.

#### Scenario: Bare-year series name with a matching show folder

- **WHEN** a file is `/tv/Doctor Who (2005)/Season 01/2005.S01E01.mkv`
- **THEN** it parses to title `Doctor Who`, year 2005, class `episode`, season 1, episode 1

### Requirement: A leading parenthesized year is the year

When a parenthesized year has no title text before it, other than release-group brackets, and no
later parenthesized year has title text before it, the filename parser SHALL read it as the release
year and take the title from the text after it. When nothing but release noise follows it, the text
before it SHALL be the title instead; show folder names follow the same reading.

#### Scenario: Year first in parentheses

- **WHEN** a file is `/movies/(2000) The Emperor's New Groove.avi`
- **THEN** it parses to title `The Emperor's New Groove`, year 2000, class `movie`

#### Scenario: Bracketed title with nothing after the year

- **WHEN** a file is `/movies/[REC] (2007).mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Release noise after the year

- **WHEN** a file is `/movies/[REC] (2007) PROPER.1080p.mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Title-then-year names are unchanged

- **WHEN** a file is `/movies/1917 (2019).mkv`, `/movies/2001 A Space Odyssey (1968).mkv` or `/movies/Blade Runner 2049 (2017).mkv`
- **THEN** they parse to titles `1917`, `2001 A Space Odyssey` and `Blade Runner 2049` with years 2019, 1968 and 2017
