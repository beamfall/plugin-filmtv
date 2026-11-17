## ADDED Requirements

### Requirement: Episode context for weak episode forms

The parser SHALL read a bare-e code or an absolute number only with episode context: a season
folder; else, unless the nearest library folder is a film one (`Movies`, `Films`), a nearest TV
folder (`TV`, `Shows`, `Anime`…), a CRC32 bracket anywhere in the name, a group bracket before
` - 001`, or a parent folder named for the series except for an `_0001` counter. A camera stem is
never a series.

#### Scenario: Camera clip

- **WHEN** the file is `VID_0001.mp4`, `TV/PXL_0003.mp4` or `Videos/DJI/DJI_0001.MP4`
- **THEN** it is not read as an episode

#### Scenario: Home video in its own folder

- **WHEN** the file is `Home/Wedding/Wedding_0002.mp4`
- **THEN** it is not read as an episode

#### Scenario: Film folder below a TV folder

- **WHEN** the file is `Media/TV/Movies/Bond - 007.mkv` or `Movies/[YTS] Agent 0011.mkv`
- **THEN** it is not read as an episode

#### Scenario: Film name ending on a number

- **WHEN** the file is `Agent 0011.mkv`, `Movie - 0420.mkv`, `Bond - 007.mkv` or `Catch-22 - 001.mkv`
- **THEN** it is not read as an episode

### Requirement: Season-by-episode code without an s

With episode context, the parser SHALL read `<season>e<episode>` (one or two season digits, two or
three episode digits) at the start of the name or after a separator, not followed by a letter or
digit and not after a channel layout such as `DD5.`. The code's own season SHALL stand, also in a
season folder naming another; in `Specials` it SHALL be 0. The rest SHALL be the `episode-title`.

#### Scenario: Code before the episode title in a season folder

- **WHEN** the file is `The IT Crowd/Season 1/1e01-Yesterday's Jam.mkv`
- **THEN** the title is `The IT Crowd`, the class is `episode`, the sequence is `1`, the index is 1
  and `episode-title` is `Yesterday's Jam`

#### Scenario: Season folder naming another season

- **WHEN** the file is `TV/Show/Season 2/Show 1e01.mkv`
- **THEN** the class is `episode`, the sequence is `1` and the index is 1

#### Scenario: No episode context

- **WHEN** the file is `Movie 2e10.mkv`
- **THEN** it is not read as an episode

#### Scenario: Channel layout

- **WHEN** the file is `TV/Movie.1080p.DD5.1e01.mkv`
- **THEN** `1e01` is not read as a season/episode token

#### Scenario: Checksum tag

- **WHEN** the file is `Show - 01 [4E01A2B3].mkv`
- **THEN** the checksum is release noise and `4E01` is not read as a season/episode token

### Requirement: Leading season and episode numbers in a season folder

When the name carries no other season/episode token, the parser SHALL read a leading pair of numbers
separated by a space, dot, underscore or dash (`3 01 From Hell`) as season and episode only inside a
season folder. A numbered folder's number SHALL equal the season; in `Specials` the season SHALL
be 0 whatever the name gives.

#### Scenario: Season folder agrees

- **WHEN** the file is `Synthetic Show/Season 3/3 01 From Hell.mkv`
- **THEN** the title is `Synthetic Show`, the class is `episode`, the sequence is `3`, the index is 1
  and `episode-title` is `From Hell`

#### Scenario: Specials

- **WHEN** the file is `Show/Specials/1 01 X.mkv`
- **THEN** the sequence is `0` and the index is 1

#### Scenario: No season folder

- **WHEN** the file is `Movies/3 10 to Yuma.mkv`
- **THEN** it is not read as an episode

### Requirement: Absolute episode number

With episode context and no season/episode token, air date or year, the parser SHALL read a non-zero
number after the series name, zero-padded to four digits (`0001`) or to three after ` - `
(`Naruto - 001`), as an absolute episode: index and absolute ordinal, no sequence. It SHALL end the
title or precede ` - <episode-title>`.

#### Scenario: Talk show numbered from 0001 in its show folder

- **WHEN** the file is `Dealbreakers Talk Show/Dealbreakers Talk Show 0001.mkv`
- **THEN** the title is `Dealbreakers Talk Show`, the class is `episode`, the index and the absolute
  ordinal are 1, there is no sequence, the display title is `E01`, and the local grouping places it
  in `Season 1`

#### Scenario: Inside a season folder

- **WHEN** the file is `Show/Season 2/Show - 001.mkv`
- **THEN** the local grouping places it in `Season 2`

#### Scenario: Anime release

- **WHEN** the file is `[Grp] Show - 001 [ABCD1234].mkv` or `Downloads/Show - 001 [ABCD1234].mkv`
- **THEN** the title is `Show` and the index and the absolute ordinal are 1

#### Scenario: Movie with a year

- **WHEN** the file is `Blade Runner 2049.mkv`
- **THEN** it is not read as an absolute-numbered episode

### Requirement: Artwork is not media

The parser SHALL refuse image files (`.jpg`, `.jpeg`, `.png`, `.gif`, `.webp`, `.bmp`, `.tbn`).

#### Scenario: Season banner

- **WHEN** the file is `Show/season01-banner.jpg`
- **THEN** the parser refuses it
