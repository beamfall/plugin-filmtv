# emby-import Specification

## Purpose
Import watch state, source users and collections from an Emby server's database into Beamfall, so a
move from Emby keeps what each person has watched.

## Requirements

### Requirement: Emby 4.10 watch state

The Emby source SHALL read watch state from the Emby 4.10 `library.db` layout, joining `UserDatas` to `MediaItems` on `UserDataKeyId`, and SHALL keep reading the older `TypedBaseItems` layout. It SHALL emit one record per item and source user that is played or has a resume position. It SHALL read `LastPlayedDateInt` as Unix seconds in UTC, treating null or 0 as no time. It SHALL skip items without a path and the container types observed in 4.10 (`type` 1, 2, 3, 4, 6, 7 and 9).

#### Scenario: A played film and a resumed episode

- **WHEN** one user played a film and resumed an episode in an Emby 4.10 `library.db`
- **THEN** the source emits a watched record for the film and a resume record for the episode

#### Scenario: A folder has user data

- **WHEN** a folder row has a played `UserDatas` row
- **THEN** the source emits no record for it

### Requirement: Emby source users

The Emby source SHALL report records per source user through `ItemsBySourceUser`. On the 4.10 layout the key SHALL be `Name#Id`, read from `users.db` beside `library.db`, and every user there SHALL appear even with no records. Without `users.db` the key SHALL be `#Id`. On the older layout, records SHALL be keyed `#<userId>` when `UserDatas` has a `userId` column, and otherwise all under `#`.

#### Scenario: Two users have state for the same file

- **WHEN** `owner` marked a film favourite and `second` resumed it
- **THEN** only `second#2` has a record for the film, and `owner#1` is reported

#### Scenario: users.db is missing

- **WHEN** `library.db` is chosen from a folder without `users.db`
- **THEN** records are keyed `#<userId>`

### Requirement: Emby ratings and favourites are not imported

The Emby source SHALL NOT advertise or report ratings, since the Emby API never stored one during observation, and SHALL NOT report favourites until a file-keyed favourite exists in the SDK (CORE-2895).

#### Scenario: Rated-only and favourite-only items

- **WHEN** a `UserDatas` row has a rating or `isFavorite` set and is neither played nor resumed
- **THEN** no record is reported for it

### Requirement: Emby collections

When collections are selected, the Emby source SHALL report each file linked to a collection (`MediaItems.type` 9) by an `ItemLinks2` row of `Type` 6 as a member named for the collection, under every source user. Positions SHALL follow `LinkOrder`, then member path, numbered from 0. The older layout reports no collections.

#### Scenario: Collection members without an order

- **WHEN** collection "Heist Night" links three files, each with `LinkOrder` 0
- **THEN** each user's records include the three files as members of "Heist Night" at positions 0, 1 and 2 in path order
