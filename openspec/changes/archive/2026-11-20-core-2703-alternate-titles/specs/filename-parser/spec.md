## ADDED Requirements

### Requirement: A dual-script title parses to its Latin title

When a parsed title is two titles, one with letters only in Latin script and one with letters
only in another script (letters common to all scripts count as neither), split by `|`, `／` or
`｜`, or with one in ASCII or full-width parentheses at the end of the other, the title SHALL be
the Latin one and the other SHALL be the `alternate-title` Media field. Any other title SHALL
stay whole and carry no alternate title.

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

### Requirement: A qualifier or a leading year is not a title split

A region or edition qualifier in parentheses SHALL NOT be taken as a second title, and a year
directly after a title bar SHALL start the second title rather than end the first.

#### Scenario: Qualifier in parentheses

- **WHEN** a file is `/movies/Бразилия (Director's Cut) (1985).mkv`
- **THEN** it parses to title `Бразилия (Director's Cut)`, year 1985, with no alternate title

#### Scenario: Year starting the Latin title

- **WHEN** a file is `/movies/Космическая одиссея | 2001 A Space Odyssey 1968.mkv`
- **THEN** it parses to title `2001 A Space Odyssey`, year 1968, alternate title `Космическая одиссея`
