## ADDED Requirements

### Requirement: Season pack

The plugin SHALL resolve a `filmtv-season` pack ref, a season Entity ID, to that season's visible
episodes in their stored order, without duplicates.

#### Scenario: Stored order

- **WHEN** a season holds episodes stored as 3, 1, 2
- **THEN** the pack holds episodes 3, 1, 2 in that order

#### Scenario: Empty ref

- **WHEN** the ref is blank
- **THEN** the resolve fails

### Requirement: Series pack

The plugin SHALL resolve a `filmtv-series` pack ref, a show Entity ID, to the episodes of every
visible season of the show: numbered seasons in ascending number, then unnumbered seasons, then
specials, where a season is specials when its `specials` flag is set or its number is 0. An episode
SHALL appear once.

#### Scenario: Specials last

- **WHEN** a show has seasons 2, 0 and 1 and a season flagged specials
- **THEN** the pack holds season 1, then season 2, then both specials seasons

### Requirement: Next unwatched pack

The plugin SHALL resolve a `filmtv-next-unwatched` pack ref `<show Entity ID>:<N>` to at most N
non-special episodes in series order that follow the last episode the requesting Profile finished,
skipping episodes finished later. A partly watched episode SHALL count as unwatched. N SHALL be
from 1 to 50.

#### Scenario: Resume after last watched

- **WHEN** the Profile finished episodes 1 and 2 and N is 2
- **THEN** the pack holds episodes 3 and 4

#### Scenario: Count out of range

- **WHEN** N is 0 or 51
- **THEN** the resolve fails before reading the store
