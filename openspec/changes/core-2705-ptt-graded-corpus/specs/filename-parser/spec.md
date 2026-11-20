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

The PTT fixture file SHALL record the PTT version and commit it came from and PTT's copyright and
MIT notice, and SHALL keep cases by a written rule that drops packs, adult and trash cases.

#### Scenario: Packs stay out

- **WHEN** any PTT fixture marks a name complete, or declares two seasons, more than three episodes, or a season with an empty episode list
- **THEN** the import leaves it out of the graded set

### Requirement: A season code's episode is read whole

The parser SHALL read every digit of an `SxxEyyyy` episode up to a resolution glued to it
(`1080p`), reading an episode above 999 as the absolute episode.

#### Scenario: Four-digit episode after a season code

- **WHEN** the parser reads `One.Piece.S01E1056.1080p.WEB.x264.mkv`
- **THEN** it parses to title `One Piece`, absolute episode 1056

#### Scenario: Resolution glued to the episode

- **WHEN** the parser reads `Show.S01E1231080p.WEB.mkv` or `One.Piece.S01E10561080p.WEB.mkv`
- **THEN** it parses to season 1, episode 123, and to absolute episode 1056
