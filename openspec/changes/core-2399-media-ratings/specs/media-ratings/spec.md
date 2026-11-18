## ADDED Requirements

### Requirement: TMDB community score

The plugin SHALL declare number Media fields `community-rating` and `community-rating-votes` and
SHALL map TMDB's `vote_average` and `vote_count` onto them for movies, series and episodes. An
episode's own document SHALL override its series' values.

#### Scenario: Movie score

- **WHEN** a TMDB movie document carries `vote_average` 7.3 and `vote_count` 1200
- **THEN** the Media stores `community-rating` 7.3 and `community-rating-votes` 1200

### Requirement: TMDB content rating

Movies SHALL map the US release certification onto `certification` and `certification-display`,
preferring theatrical, then digital, then the first US release. Series SHALL map the US TV content
rating onto `certification-display` only and SHALL NOT write `certification`.

#### Scenario: Movie with a US theatrical rating

- **WHEN** a movie's US theatrical release is certified "PG-13"
- **THEN** the Media stores `certification` and `certification-display` "PG-13"

#### Scenario: Series with a TV rating

- **WHEN** a TMDB series document carries a US content rating "TV-MA"
- **THEN** the Media stores `certification-display` "TV-MA" and no `certification`

#### Scenario: No US certification

- **WHEN** a movie has no US release certification
- **THEN** no certification field is stored
