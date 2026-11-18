## MODIFIED Requirements

### Requirement: Episode title field

The parser SHALL emit the text between the episode token and the first release tag or bracket as the
Media field `episode-title`, trimmed of separators, and SHALL emit no such field when that text is
empty or the file is not an episode. Release tags include dynamic-range, audio and streaming-source
tags (`DV`, `DoVi`, `HDR`, `HDR10+`, `SDR`, `Atmos`, `ATVP`); one that is also a word in a title
(`DV`, `HDR`, `Atmos`) ends the title only before more release tags or as the last word. The bundle
SHALL declare `episode-title` as a searchable text Media field.

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

#### Scenario: Dynamic-range tag right after the episode token

- **WHEN** the file is `ted.lasso.s03e08.dv.2160p.web.h265-ggez.mkv`
- **THEN** no `episode-title` field is emitted

#### Scenario: Title before streaming and dynamic-range tags

- **WHEN** the file is `Ted.Lasso.S03E02.I.Dont.Want.to.Go.to.Chelsea.2160p.ATVP.WEB-DL.DDP5.1.HDR.H.265-NTb.mkv`
- **THEN** `episode-title` is `I Dont Want to Go to Chelsea`

#### Scenario: Tag word inside a real title

- **WHEN** the file is `Show.S01E01.The.HDR.Story.1080p.mkv`
- **THEN** `episode-title` is `The HDR Story`
