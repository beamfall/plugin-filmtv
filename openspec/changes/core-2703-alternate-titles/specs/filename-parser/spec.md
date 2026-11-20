## ADDED Requirements

### Requirement: A dual-script title parses to its Latin title

When a parsed title is two titles, one only in Latin script and one with letters of another
script, split by `|`, `／` or `｜`, or with one in parentheses at the end of the other, the title
SHALL be the Latin one and the other SHALL be the `alternate-title` Media field. Any other title
SHALL stay whole and carry no alternate title.

#### Scenario: Bar-separated titles

- **WHEN** a file is `/movies/Голубая волна | Blue Crush (2002) DVDRip.mkv`
- **THEN** it parses to title `Blue Crush`, year 2002, alternate title `Голубая волна`

#### Scenario: Latin title in parentheses

- **WHEN** a file is `/movies/О мышах и людях (Of Mice and Men) 1992.mkv`
- **THEN** it parses to title `Of Mice and Men`, year 1992, alternate title `О мышах и людях`

#### Scenario: Non-Latin title only

- **WHEN** a file is `/movies/Жихарка (2007).mkv`
- **THEN** it parses to title `Жихарка`, year 2007, with no alternate title

#### Scenario: Two Latin titles

- **WHEN** a file is `/movies/Star Wars (Special Edition) 1977.mkv`
- **THEN** it parses to title `Star Wars (Special Edition)`, with no alternate title
