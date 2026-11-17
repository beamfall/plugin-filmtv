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

The scorer SHALL take no episode penalty for a candidate that carries neither season nor episode.
For an episode, a series title equal to the candidate's but for a trailing country qualifier (`UK`,
`US`, `(US)`) SHALL score 0.9 with no year needed; a candidate carrying the same qualifier SHALL
score 1.0 and one carrying another SHALL not match.

#### Scenario: Country-qualified show

- **WHEN** the parsed episode's series is `The Traitors UK` and the only candidate is `The Traitors`
- **THEN** the score is 0.9 and the match auto-applies

#### Scenario: Two national versions under one title

- **WHEN** the parsed series is `The Office (UK)` and two candidates are both titled `The Office`
- **THEN** the two tie and neither auto-applies
