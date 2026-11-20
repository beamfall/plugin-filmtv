## ADDED Requirements

### Requirement: Unpadded absolute episode numbers

With episode context, the filename parser SHALL read two to four digits after a spaced hyphen at the
end of the name, before release noise, or before ` - ` and an episode title, as the absolute episode
number, with an optional release revision (`v` and one or two digits) after the digits. It SHALL
return class `episode`, the series name as the title, and the number as both `Index` and
`AbsoluteOrdinal`.

#### Scenario: Two digits in a season folder

- **WHEN** a file is `/tv/Mob Psycho 100/Season 01/Mob Psycho 100 - 09 [1080p].mkv`
- **THEN** it parses to title `Mob Psycho 100`, class `episode`, absolute episode 9

#### Scenario: Three digits before an episode title

- **WHEN** a file is `/tv/Naruto Shippuden/Naruto Shippuden - 107 - Strange Bedfellows.mkv`
- **THEN** it parses to title `Naruto Shippuden`, absolute episode 107

#### Scenario: Four digits in a show folder

- **WHEN** a file is `/tv/One Piece/One Piece - 1071.mkv`
- **THEN** it parses to title `One Piece`, absolute episode 1071

#### Scenario: Group bracket and release revision

- **WHEN** a file is `/Downloads/[SubsPlease] Frieren - 12v2 (1080p) [ABCD1234].mkv`
- **THEN** it parses to title `Frieren`, absolute episode 12

### Requirement: A year does not hide an absolute episode number

A year in the series name or in the episode title SHALL NOT prevent the absolute episode reading, and
a year in the series name SHALL be returned as the year. A show folder named like the series SHALL
NOT count as episode context for a series name with a year, because a film's own folder has the same
shape. A dashed four-digit number from 1900 to 2099 SHALL be read as a year, not as an episode number.

#### Scenario: Year in the series name

- **WHEN** a file is `/anime/Hunter x Hunter (2011)/Hunter x Hunter (2011) - 12.mkv`
- **THEN** it parses to title `Hunter x Hunter`, year 2011, absolute episode 12

#### Scenario: Year in the episode title

- **WHEN** a file is `/tv/Show - 12 - Class of 1999.mkv`
- **THEN** it parses to title `Show`, no year, absolute episode 12, episode title `Class of 1999`

#### Scenario: Film in its own folder

- **WHEN** a file is `/incoming/Heat (1995)/Heat (1995) - 1080.mkv`
- **THEN** it parses to title `Heat`, class `movie`, year 1995

#### Scenario: Dashed year

- **WHEN** a file is `/tv/Show/Show - 2015.mkv`
- **THEN** it parses to title `Show`, year 2015, and no episode

### Requirement: Film numbers after a hyphen stay in the title

The parser SHALL NOT read a single digit after a spaced hyphen as an absolute episode, and SHALL NOT
read any number after a spaced hyphen as one without episode context or when the nearest library
folder above the file is a film library, a season folder below it included.

#### Scenario: Sequel number

- **WHEN** a file is `/Videos/Rocky/Rocky - 2.mkv` or `Movie - 2.mkv`
- **THEN** it parses as a film with the number in its title

#### Scenario: Film library

- **WHEN** a file is `/media/Movies/Saw - 10.mkv`
- **THEN** it parses to title `Saw - 10`, class `movie`

#### Scenario: Season folder in a film library

- **WHEN** a file is `/media/Movies/Specials/Saw - 10.mkv`
- **THEN** it parses to title `Saw - 10`, class `movie`
