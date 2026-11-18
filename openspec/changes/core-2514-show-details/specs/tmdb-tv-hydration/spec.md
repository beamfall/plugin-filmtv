## ADDED Requirements

### Requirement: TMDB hydrates the show's details

The `tmdb-tv` scraper SHALL hydrate the existing `show` Entity holding an identified episode from the
series document root with its summary, first and last air dates, status, primary network, US content
rating, TMDB community score and vote count, poster and backdrop.

#### Scenario: Series details on the show

- **WHEN** an episode of a series whose document lists a backdrop, `networks[0].name` "Netflix", a US
  content rating and a first air date in 2024 is identified
- **THEN** the show Entity receives the backdrop, `network` "Netflix", the US rating as
  `certification-display` and `first-air-date` in 2024

### Requirement: A matched show takes the TMDB title

The `tmdb-tv` show container map SHALL declare the series `name` as the show's display title, so a
matched show replaces its folder spelling and keeps it only when the document has no name.

#### Scenario: Apostrophes and accents

- **WHEN** a show built from the folder "90 Day Fiance" is matched to the series "90 Day Fiancé"
- **THEN** the show's title is "90 Day Fiancé" and its identity is unchanged

### Requirement: TMDB links the show to its genres and cast

The `tmdb-tv` show container map SHALL link the show to every genre (`in-genre`) and cast member
(`credit`) the same series lookup hydrates, in billing order.

#### Scenario: Show genres and cast

- **WHEN** an episode is identified against a series document listing genres and cast
- **THEN** the show Entity relates to those genres and people
