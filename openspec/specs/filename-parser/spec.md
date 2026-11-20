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
as the release year. Without that context the name SHALL parse as a film name, its year read by the
general rules.

#### Scenario: Spaced series year

- **WHEN** a file is `/tv/Space 1999/Season 01/Space 1999 0001.mkv`
- **THEN** it parses to series `Space 1999`, no year, absolute episode 1

#### Scenario: No context

- **WHEN** a file is `/incoming/Space 1999 0001.mkv`
- **THEN** it parses to the film `Space`, year 1999

### Requirement: Release-group brackets and checksums are never film title text

Leading release-group brackets MAY be separated by spaces, dots, underscores or hyphens, and years
inside any of them SHALL NOT count. A film title SHALL drop its leading release-group brackets when
only a year-named title and a release year follow them, and SHALL keep them otherwise (`[REC] 2`). A CRC32 checksum
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

After a leading parenthesized year, a scene tag (`PROPER`, `REPACK`, `RERIP`) that only checksum
brackets and release tags follow SHALL be release noise. When a bracketed title stands before the
year, that title stays the title. When nothing stands before the year, the name SHALL have no title
and SHALL NOT parse. Further scene tags, a tag split in two (`H.264`) and a release group
hyphenated onto the last tag are part of that noise; an episode marker ends it. A release
word that can be a title directly after the scene tag, or any word after the noise that is not
release noise, SHALL keep the scene tag as the title, with or without a checksum. A show folder SHALL follow the same rule.

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

#### Scenario: Scene tag before a title-capable word without a checksum

- **WHEN** a file is `/movies/[Group] (2013) Proper Uncut.mkv`
- **THEN** it parses to title `Proper`, year 2013, its trailing `Uncut` an edition word

#### Scenario: Title word after the noise

- **WHEN** a file is `/movies/(2007) PROPER.1080p.Story.mkv`
- **THEN** it parses to title `PROPER`, year 2007

#### Scenario: Scene noise in a show folder

- **WHEN** a file is `/tv/(2007) PROPER.[DEADBEEF].1080p/Season 01/S01E01.mkv`
- **THEN** it does not parse

#### Scenario: Stacked scene tags

- **WHEN** a file is `/movies/[REC] (2007) PROPER.REPACK.1080p.mkv`
- **THEN** it parses to title `[REC]`, year 2007

#### Scenario: Ordinary release word

- **WHEN** a file is `/movies/[Group] (2013) DV [DEADBEEF].mkv`
- **THEN** it parses to title `DV`, year 2013

### Requirement: A dashed episode can follow a dashed year

When a dashed number in the year range (` - 2011`) is directly followed by another dashed number, the parser
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

#### Scenario: Dashed number in later metadata

- **WHEN** a file is `/tv/Show - 2011 - [Disc - 12 - Bonus].mkv`
- **THEN** it parses to title `Show`, year 2011, no episode

### Requirement: A checksum folder names no series

A series name taken from a show folder SHALL NOT include a CRC32 checksum bracket, and a checksum
SHALL NOT turn a year-shaped series name into a year. A folder that holds nothing else SHALL name no
series.

#### Scenario: Checksum show folder

- **WHEN** a file is `/tv/[DEADBEEF]/Season 01/S01E01.mkv`
- **THEN** it does not parse

#### Scenario: Checksum before a year-shaped series name

- **WHEN** a file is `/tv/[DEADBEEF] 1923/Season 01/S01E01.mkv`
- **THEN** it parses to series `1923`, no year, season 1 episode 1

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
