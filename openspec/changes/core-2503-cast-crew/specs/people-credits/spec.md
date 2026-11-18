## ADDED Requirements

### Requirement: TMDB cast credits carry character and billing

The `tmdb-movie` and `tmdb-tv` `credits.cast` EntityMaps SHALL write the element's `character` and
its TMDB `order` as `billing` on the `credited-in` relation, and SHALL order cast credits by
`billing`.

#### Scenario: Lead actor

- **WHEN** a TMDB credits document lists Russell Crowe with `character` "John Nash" and `order` 0
- **THEN** the `credited-in` relation carries `character` "John Nash" and `billing` "0" and sorts first

### Requirement: TMDB key crew credits

The `tmdb-movie` and `tmdb-tv` scrapers SHALL map `credits.crew` elements whose `job` is Director,
Screenplay, Writer, Producer, Original Music Composer or Director of Photography to `credited-in`
Person credits carrying `job` and `department`, ordered after every cast credit. Other crew jobs
SHALL NOT be imported.

#### Scenario: Director

- **WHEN** a credits document lists Ron Howard with `job` "Director" and `department` "Directing"
- **THEN** a `credited-in` relation carries `job` "Director" and `department` "Directing"

#### Scenario: Other crew job

- **WHEN** a crew element's `job` is "Casting"
- **THEN** no credit is written for it
