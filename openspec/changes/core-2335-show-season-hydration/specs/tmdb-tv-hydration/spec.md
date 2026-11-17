## ADDED Requirements

### Requirement: TMDB hydrates the show and season holding an episode

The `tmdb-tv` scraper SHALL hydrate the existing `show` Entity holding an identified episode from the
series document root (poster, overview and first air date), and the existing `season` Entity from the
series document's season whose number is the episode's parsed season (poster, overview and air date).
It SHALL NOT create show or season Entities.

#### Scenario: Season poster from the series document

- **WHEN** `S01E01` of a series is identified and the series document lists season 1 with a poster
- **THEN** the season Entity holding the episode receives that poster and overview, and the show
  receives the series poster and overview

### Requirement: Episodes take their own TMDB document

The `tmdb-tv` scraper SHALL fetch `/tv/{id}/season/{sequence}/episode/{index}` for an episode with a
parsed season and episode number, and map its overview to the summary, its air date to the year and
its still to the thumb, overriding the series values. The episode SHALL keep the parser's title.

#### Scenario: Episode year from its air date

- **WHEN** `S03E02` of a series that premiered in 2019 is identified and the episode aired in 2021
- **THEN** the episode's year is 2021 and its summary is the episode overview
