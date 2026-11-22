## ADDED Requirements

### Requirement: TMDB collections build film series

The `tmdb-movie` scraper SHALL hydrate one `film-series` Entity per TMDB collection in a Library from
the movie document's `belongs_to_collection`, titled with the collection name and carrying its
`tmdb-id`, poster and backdrop, and SHALL link every identified member film to it through the
single-holder `film-series-film` relation.

#### Scenario: Two films in one collection

- **WHEN** two films whose documents name the same `belongs_to_collection` are identified
- **THEN** the Library has one `film-series` Entity with the collection's name, `tmdb-id`, poster and
  backdrop, linked to both films

#### Scenario: A film with no collection

- **WHEN** a film whose document has a null `belongs_to_collection` is identified
- **THEN** no `film-series` Entity is created for it

### Requirement: Film series membership follows the match

A film's `film-series` link SHALL follow its current identification: re-identifying it against a
document with no collection removes the link, and against another collection moves it to that series.

#### Scenario: A film moves to another collection

- **WHEN** a film linked to one series is re-identified against a document naming another collection
- **THEN** it is linked only to the other series, and the first series no longer lists it

### Requirement: Film series list members in release order

A `film-series` SHALL list its member films in release-year order, oldest first.

#### Scenario: Identified out of order

- **WHEN** the 2003 film of a collection is identified before the 1999 film
- **THEN** the series lists the 1999 film first
