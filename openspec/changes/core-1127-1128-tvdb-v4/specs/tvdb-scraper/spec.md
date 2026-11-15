## ADDED Requirements

### Requirement: The TheTVDB scraper follows the v4 API

The `tvdb-series` scraper SHALL search with the year when one is known, read the extended record
with translations, map title and summary from the configured language with original-language
fallback, resolve IMDb and TMDB ids through `/search/remoteid/{id}`, and map extra artwork by type
ids read from `/artwork/types`.

#### Scenario: Same-titled series

- **WHEN** a file carries a title and year
- **THEN** the search request filters by that year

#### Scenario: Configured language

- **WHEN** the metadata language is `eng` and the record has an English translation
- **THEN** the title and summary are the English ones, and the original-language values otherwise

#### Scenario: Artwork type ids

- **WHEN** the extended record lists artworks by numeric type
- **THEN** backdrop, banner and logo are chosen by the ids `/artwork/types` gives for those slugs

### Requirement: The TheTVDB credential is presented as a subscriber PIN

The scraper SHALL label its AuthRef credential as the TheTVDB subscriber PIN and link to TheTVDB's
subscription page.

#### Scenario: Console provider key

- **WHEN** the owner opens the TheTVDB provider key
- **THEN** the field is labelled as the subscriber PIN with a "Subscribe at thetvdb.com" link
