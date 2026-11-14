## ADDED Requirements

### Requirement: TVmaze provider

The bundle SHALL declare a `tvmaze-show` scraper that looks a show up on TVmaze by its TVmaze id,
declares provider-ID lookups for `tvdb` and `imdb`, implies class `episode`, sends no credential,
spaces its requests at least 500 ms apart and declares an attribution that credits TVmaze and links
to `https://www.tvmaze.com`.

#### Scenario: Lookup by TVmaze id

- **WHEN** the scraper looks up TVmaze id `82`
- **THEN** it requests `https://api.tvmaze.com/shows/82?embed=episodes` with no credential
- **AND** maps the show's name to the title, its premiere date to the year and its original image
  to the poster

#### Scenario: Provider-ID hint

- **WHEN** the parsed media carries a TheTVDB id
- **THEN** the scraper's `tvdb` lookup template is `/lookup/shows?thetvdb={id}`

### Requirement: AniList providers

The bundle SHALL declare `anilist-series` (class `episode`) and `anilist-movie` (class `movie`)
scrapers that search and look up anime through AniList's GraphQL API, present the operator's
`anilist-access-token` as a bearer token, are omitted when that credential is unset and space
their requests at least 2 s apart.

#### Scenario: No token

- **WHEN** no `anilist-access-token` is configured
- **THEN** both AniList scrapers are optional-auth scrapers and send no request

#### Scenario: Series search

- **WHEN** `anilist-series` searches for `Pioneer One`
- **THEN** it posts a GraphQL query for anime in TV, short, web, OVA and special formats
- **AND** each candidate carries the AniList id, the romaji title, the start year and the cover
  image

### Requirement: Absolute episode mapping

The plugin SHALL map an absolute episode number to a season and episode by counting a series'
regular episodes in season and episode order, ignoring specials and season 0, and SHALL report no
mapping when the number is not positive or exceeds the number of regular episodes.

#### Scenario: Second-season episode

- **WHEN** season 1 has 12 regular episodes and the absolute number is 13
- **THEN** the mapping is season 2, episode 1

#### Scenario: Specials are skipped

- **WHEN** a special airs between season 1 episode 12 and season 2 episode 1
- **THEN** absolute 13 still maps to season 2, episode 1

#### Scenario: Past the end

- **WHEN** the series has 24 regular episodes and the absolute number is 25
- **THEN** there is no mapping
