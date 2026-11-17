## ADDED Requirements

### Requirement: TheTVDB is optional

The `tvdb-series` scraper SHALL declare its credential optional, so a library without a TheTVDB PIN
does not list it as a missing required key and identification skips it. This does not change which
scraper identifies TV: `tmdb-tv` comes first either way.

#### Scenario: No TheTVDB PIN

- **WHEN** identification runs with no TheTVDB PIN
- **THEN** `tvdb-series` is skipped and is not reported as a missing key

### Requirement: Title normalisation

The match scorer SHALL compare titles with apostrophes (`'`, `’`, `‘`, `ʼ`, backtick) removed, a
lone `s` joined to the word before it, and `&` read as `and`.

#### Scenario: Scene name without an apostrophe

- **WHEN** the parsed title is `A Mothers Son` or `Ocean_s_Eleven` and the candidate title is
  `A Mother's Son` or `Ocean's Eleven`
- **THEN** the titles score as an exact match

#### Scenario: Ampersand

- **WHEN** the parsed title is `Key and Peele` and the candidate title is `Key & Peele`
- **THEN** the titles score as an exact match

### Requirement: Show-level TV candidates auto-apply

A candidate carrying neither season nor episode SHALL lose only 0.01, so an exact title still
auto-applies but ranks below an agreeing episode candidate. A country qualifier is `(US)`-style on
the candidate, and on the parse also bare all-caps (`UK`). The same qualifier on both SHALL score
as exact; on one side only, the title scores 0.9 and SHALL NOT auto-apply without a year, because a
candidate carries no origin country.

#### Scenario: Show-level candidate

- **WHEN** the parsed episode's series is `Key and Peele` and the only candidate is `Key & Peele`
- **THEN** the score is 0.99 and the match auto-applies

#### Scenario: Qualifier on one side only

- **WHEN** the file is `The.Office.UK.S01E01.mkv` and the candidates are `The Office` (2001) and
  `The Office` (2005), or the parse is `Shameless` and the candidates `Shameless (US)` and
  `Shameless (UK)`
- **THEN** neither candidate auto-applies

#### Scenario: A word that looks like a country

- **WHEN** the parsed series is `This Is` and the candidate is `This Is Us`
- **THEN** `Us` is not read as a qualifier and the match does not auto-apply
