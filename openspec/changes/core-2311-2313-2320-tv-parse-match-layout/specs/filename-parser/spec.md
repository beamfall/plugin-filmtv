## ADDED Requirements

### Requirement: Season-by-episode code without an s

The parser SHALL read `<season>e<episode>` (one or two season digits, two or three episode digits)
as the season/episode token when it stands at the start of the name or after a separator and is not
followed by a letter or digit. The text after it SHALL be the `episode-title`.

#### Scenario: Code before the episode title in a season folder

- **WHEN** the file is `The IT Crowd/Season 1/1e01-Yesterday's Jam.mkv`
- **THEN** the title is `The IT Crowd`, the class is `episode`, the sequence is `1`, the index is 1
  and `episode-title` is `Yesterday's Jam`

#### Scenario: Checksum tag

- **WHEN** the file is `Show - 01 [4E01A2B3].mkv`
- **THEN** `4E01` inside the checksum is not read as a season/episode token

### Requirement: Leading season and episode numbers in a season folder

When the name carries no other season/episode token, the parser SHALL read a leading pair of numbers
separated by a space, dot, underscore or dash (`3 01 From Hell`) as season and episode only when the
file sits in a season folder and that folder's number, when it carries one, equals the season.

#### Scenario: Season folder agrees

- **WHEN** the file is `Synthetic Show/Season 3/3 01 From Hell.mkv`
- **THEN** the title is `Synthetic Show`, the class is `episode`, the sequence is `3`, the index is 1
  and `episode-title` is `From Hell`

#### Scenario: No season folder

- **WHEN** the file is `Movies/3 10 to Yuma.mkv`
- **THEN** it is not read as an episode

### Requirement: Trailing absolute episode number

When the name carries no season/episode token, no air date and no year, the parser SHALL read a
number ending the title as an absolute episode index when it is zero-padded to four digits
(`0001`), or zero-padded to three digits and set off by ` - ` (`Naruto - 001`). The title
SHALL be the text before it, the class `episode`, and no sequence SHALL be invented. The local
grouping SHALL place such an episode in Season 1 and its display code SHALL be `E` and the index
padded to two digits.

#### Scenario: Talk show numbered from 0001

- **WHEN** the file is `Dealbreakers Talk Show 0001.mkv`
- **THEN** the title is `Dealbreakers Talk Show`, the class is `episode`, the index is 1, there is no
  sequence, and the display title is `E01`

#### Scenario: Movie with a year and a number

- **WHEN** the file is `Blade Runner 2049.mkv`
- **THEN** it is not read as an absolute-numbered episode

#### Scenario: Three-digit number without a dash

- **WHEN** the file is `Agent 007.mkv`
- **THEN** it is not read as an absolute-numbered episode
