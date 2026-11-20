## ADDED Requirements

### Requirement: Unpadded absolute episode numbers

With episode context, the filename parser SHALL read two to four digits after a spaced hyphen at the
end of the name, before release noise, or before ` - ` and an episode title, as the absolute episode
number, with an optional release revision `v<n>` after the digits. It SHALL return class `episode`,
the series name as the title, and the number as both `Index` and `AbsoluteOrdinal`.

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

### Requirement: Film numbers after a hyphen stay in the title

The parser SHALL NOT read a single digit after a spaced hyphen as an absolute episode, and SHALL NOT
read any number after a spaced hyphen as one without episode context or in a film library.

#### Scenario: Sequel number

- **WHEN** a file is `/Videos/Rocky/Rocky - 2.mkv` or `Movie - 2.mkv`
- **THEN** it parses as a film with the number in its title

#### Scenario: Film library

- **WHEN** a file is `/media/Movies/Saw - 10.mkv`
- **THEN** it parses to title `Saw - 10`, class `movie`
