# filename-parser Specification

## Purpose
How the Film & TV parser turns a media file path into a title, class, year, season and episode
numbers, and the `episode-title` Media field that Core indexes for local files.

## Requirements

### Requirement: Clean series title for episode files

The parser SHALL report the series name as the parsed title of an episode file, with no leading or
trailing separator characters and no leading release-group bracket.

#### Scenario: Dash-separated name

- **WHEN** the file is `Pioneer One - S01E01 - Earthfall.mkv`
- **THEN** the title is `Pioneer One`, the class is `episode`, the sequence is `1` and the index is 1

#### Scenario: Dotted scene name

- **WHEN** the file is `Pioneer.One.S01E01.Earthfall.1080p.x264-GRP.mkv`
- **THEN** the title is `Pioneer One`

#### Scenario: Series name only in the folder

- **WHEN** the file is `Pioneer One (2010)/Season 01/S01E01 - Earthfall.mkv`
- **THEN** the title is `Pioneer One` and the year is 2010

#### Scenario: No series name anywhere

- **WHEN** the file is `3x07.Some.Show.mkv` outside a season folder
- **THEN** the parser refuses the file

### Requirement: Episode title field

The parser SHALL emit the text between the episode token and the first release tag or bracket as the
Media field `episode-title`, trimmed of separators, and SHALL emit no such field when that text is
empty or the file is not an episode. The bundle SHALL declare `episode-title` as a searchable text
Media field.

#### Scenario: Title before release tags

- **WHEN** the file is `Pioneer.One.S01E01.Earthfall.1080p.x264-GRP.mkv`
- **THEN** `episode-title` is `Earthfall`

#### Scenario: Release tag that is also a word

- **WHEN** the file is `Show.S01E02.Internal.Affairs.720p.HDTV.mkv`
- **THEN** `episode-title` is `Internal Affairs`

#### Scenario: Weak release word before a resolution only

- **WHEN** the file is `Show.S01E01.The.Web.1080p.mkv`
- **THEN** `episode-title` is `The Web`

#### Scenario: Release tag before a release group

- **WHEN** the file is `Show.S01E01.Pilot.PROPER-GRP.mkv`
- **THEN** `episode-title` is `Pilot`

#### Scenario: No episode title

- **WHEN** the file is `Pioneer One S01E01.mkv`
- **THEN** no `episode-title` field is emitted

### Requirement: Multi-episode range

The parser SHALL report the first episode of a multi-episode chain as the index and the last as the
index end.

#### Scenario: Three-episode chain

- **WHEN** the file is `Show.Name.S01E01-E02-E03.mkv`
- **THEN** the index is 1 and the index end is 3

#### Scenario: Season-by-episode chain

- **WHEN** the file is `Show Name - 1x01-1x02 - Pilot.mkv`
- **THEN** the index is 1, the index end is 2 and `episode-title` is `Pilot`

#### Scenario: Three-token season-by-episode chain

- **WHEN** the file is `Show.Name.1x01-1x02-1x03.Pilot.mkv`
- **THEN** the index is 1, the index end is 3 and `episode-title` is `Pilot`

#### Scenario: Chain that crosses seasons

- **WHEN** the file is `Show.S01E01-S02E03.Pilot.mkv`
- **THEN** the sequence is `1`, the index is 1, there is no index end and `episode-title` is `Pilot`

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
