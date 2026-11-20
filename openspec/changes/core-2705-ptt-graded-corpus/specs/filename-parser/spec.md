## ADDED Requirements

### Requirement: The parser is graded on an imported third-party corpus

The parser SHALL be graded on the title, year, season and episode that PTT's library-shaped
fixtures declare, and each field SHALL have a documented floor rate at or below the current rate.

#### Scenario: A resolved case regresses

- **WHEN** a parser change makes a PTT case's title, year, season or episode differ from the fixture, and that field is not marked unresolved
- **THEN** the graded corpus test fails naming the case and field

#### Scenario: A gap is fixed

- **WHEN** a parser change makes a marked unresolved field parse as the fixture declares
- **THEN** the graded corpus test fails until the marks are rewritten, and the field's rate rises

### Requirement: The imported corpus records its source and rule

The PTT fixture file SHALL record the PTT version, commit and licence it came from, and SHALL keep
cases by a written rule that drops packs, adult and trash cases.

#### Scenario: Packs stay out

- **WHEN** a PTT fixture declares two seasons, a season with no episode, or more than three episodes
- **THEN** the import leaves it out of the graded set

### Requirement: A season code's episode is read whole and a channel layout is no season code

The parser SHALL read every digit of an `SxxEyyyy` episode, reading one above 999 as the absolute
episode, and SHALL NOT read an `NxM` code whose season follows a lone digit and a dot as a
season and episode.

#### Scenario: Four-digit episode after a season code

- **WHEN** the parser reads `One.Piece.S01E1056.1080p.WEB.x264.mkv`
- **THEN** it parses to title `One Piece`, absolute episode 1056

#### Scenario: Channel layout

- **WHEN** the parser reads `Escaflowne (2000) (BDRip x265 AC3 5.1x2+2.0x3).mkv`
- **THEN** it parses to the film `Escaflowne` (2000), and `Doctor.Who.2005.8x11.Dark.Water.720p.mkv` still parses to season 8, episode 11
