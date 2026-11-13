## ADDED Requirements

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
