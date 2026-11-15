## ADDED Requirements

### Requirement: Date-named episodes

When a file name carries no season/episode token, the parser SHALL read a calendar-valid air date
written `YYYY-MM-DD`, or with `.`, `_` or space separators, as the episode token. The file SHALL be
classified `episode`, its parsed title SHALL be the text before the date, it SHALL carry no sequence
or index, and the text after the date SHALL be its `episode-title`. The parser SHALL emit the air date
as the date Media field `air-date` through `ParseTypedMediaFields`, together with `episode-title`.

#### Scenario: Daily file

- **WHEN** the file is `Synthetic Daily 2023-03-01.mkv`
- **THEN** the title is `Synthetic Daily`, the class is `episode`, there is no sequence, index or year,
  and `air-date` is 2023-03-01

#### Scenario: Dotted daily file with an episode title

- **WHEN** the file is `Synthetic.Daily.2023.03.02.Guest.Night.720p.mkv`
- **THEN** the title is `Synthetic Daily`, `air-date` is 2023-03-02 and `episode-title` is
  `Guest Night`

#### Scenario: Not a calendar date

- **WHEN** the file is `Synthetic Daily 2023-13-01.mkv`
- **THEN** it is not read as a dated episode

#### Scenario: Season/episode token wins

- **WHEN** the file is `Synthetic Show S01E02 2023-03-01.mkv`
- **THEN** the sequence is `1`, the index is 2 and no `air-date` is emitted

#### Scenario: Identification keeps the parsed air date

- **WHEN** a date-named episode is identified through a series scraper
- **THEN** its `air-date` stays the parsed date, because no scraper maps the series premiere onto
  `air-date`
