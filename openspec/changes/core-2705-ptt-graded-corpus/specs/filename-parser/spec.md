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
