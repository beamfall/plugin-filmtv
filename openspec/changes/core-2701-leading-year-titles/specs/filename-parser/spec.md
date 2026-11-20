## ADDED Requirements

### Requirement: A leading standalone year is title text

The filename parser SHALL NOT read a standalone year as the release year when no title text comes
before it, or when only separators stand between it and a later year. That year SHALL stay in the
title, and the next qualifying year, if any, SHALL be the release year.

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

### Requirement: A leading parenthesized year is the year

When a parenthesized year has no title text before it, other than release-group brackets, and no
later parenthesized year has title text before it, the filename parser SHALL read it as the release
year and take the title from the text after it.

#### Scenario: Year first in parentheses

- **WHEN** a file is `/movies/(2000) The Emperor's New Groove.avi`
- **THEN** it parses to title `The Emperor's New Groove`, year 2000, class `movie`

#### Scenario: Title-then-year names are unchanged

- **WHEN** a file is `/movies/1917 (2019).mkv`, `/movies/2001 A Space Odyssey (1968).mkv` or `/movies/Blade Runner 2049 (2017).mkv`
- **THEN** they parse to titles `1917`, `2001 A Space Odyssey` and `Blade Runner 2049` with years 2019, 1968 and 2017
