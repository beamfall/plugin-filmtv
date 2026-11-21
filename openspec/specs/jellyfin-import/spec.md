# jellyfin-import Specification

## Purpose
Import watch state from a Jellyfin 12 server's `jellyfin.db`, one record per item and source user, and refuse an older Jellyfin `library.db` with an error telling the operator to upgrade first.

## Requirements

### Requirement: Jellyfin 12 watch state

The Jellyfin source SHALL read watch state from a Jellyfin 12 `jellyfin.db`, joining `UserData` to `BaseItems` on the item ID. It SHALL emit one record per item and source user that is played or has a resume position, and SHALL skip folders and items without a path.

#### Scenario: Two users have state for the same file

- **WHEN** one user finished an item and another stopped partway through it
- **THEN** the source emits one record for each user

#### Scenario: A folder has user data

- **WHEN** a series folder has a played row
- **THEN** the source emits no record for it

### Requirement: Jellyfin data-key copies

When one user has several `UserData` rows for one item, one per data key, the Jellyfin source SHALL emit one record. It SHALL use the row with the latest last-played instant, then a played row, then the larger resume position, then the lowest data key. It SHALL compare instants after parsing, ranking an absent or unreadable time last.

#### Scenario: One user's state is stored under several data keys

- **WHEN** a user's played state for a film is stored under three data keys
- **THEN** the source emits one record for that user and film

#### Scenario: Two spellings of one instant

- **WHEN** a played copy has `LastPlayedDate` `2026-11-21 13:00:00` and an unplayed copy has `2026-11-21 13:00:00.0000000`
- **THEN** the source emits the played record

### Requirement: Jellyfin last-played time

The Jellyfin source SHALL read `LastPlayedDate` as UTC in Jellyfin's stored form: a space between date and time and up to seven fractional digits. An unreadable value SHALL leave that record's last-played time empty and SHALL NOT end the import.

#### Scenario: Stored timestamp form

- **WHEN** a row's `LastPlayedDate` is `2026-11-21 13:40:27.0279689`
- **THEN** the record's last-played time is 13:40:27.0279689 UTC on 2026-11-21

#### Scenario: Malformed timestamp

- **WHEN** a row's `LastPlayedDate` is not a timestamp
- **THEN** the record is emitted with an empty last-played time

### Requirement: Jellyfin database recognition

Before it reads rows, the Jellyfin source SHALL check that the file has the Jellyfin 12 tables and columns it reads. A legacy Jellyfin `library.db` SHALL be refused with an error telling the operator to upgrade to Jellyfin 12, then choose `jellyfin.db`. Another SQLite database SHALL be refused naming the first missing table or column; a file that is not SQLite, with the read error. The Emby source SHALL keep reading the legacy `library.db` layout.

#### Scenario: The operator chooses a Jellyfin 10.10 library.db

- **WHEN** the Jellyfin source opens a database that has `TypedBaseItems` and no `BaseItems`
- **THEN** the import fails with an error naming Jellyfin 12 and `jellyfin.db`

#### Scenario: The operator chooses an unrelated SQLite file

- **WHEN** the Jellyfin source opens a database that has the Jellyfin 12 `BaseItems` table and no `UserData`
- **THEN** the import fails with an error naming the missing `UserData` table

### Requirement: Jellyfin source users

The Jellyfin source SHALL report records per source user through `ItemsBySourceUser`, keyed `Username#Id` from the `Users` table. Every user in `Users` SHALL appear, including users with no records, so the operator can map or skip each one.

#### Scenario: Two users have state for the same file

- **WHEN** `owner` finished a film and `second` stopped partway through it
- **THEN** the film's watched record is under `owner#<owner id>` and its resume record under `second#<second id>`

#### Scenario: A user has no state

- **WHEN** a user has no `UserData` rows
- **THEN** that user's key is reported with no records

### Requirement: Jellyfin ratings

When ratings are selected, the Jellyfin source SHALL report a user's `UserData.Rating` on a scale of 10, taken from the best data-key copy that has a rating, ranked as for watch state. A rated item that is neither played nor resumed SHALL carry the rating and no watch state.

#### Scenario: Rated-only film

- **WHEN** a user rated a film 8.5 and never played it
- **THEN** that user's record for the film has rating 8.5 of 10 and no watch state

#### Scenario: Ratings not selected

- **WHEN** a run selects watch state only
- **THEN** no record carries a rating and rated-only items are not reported

### Requirement: Jellyfin favourites are not imported

The Jellyfin source SHALL NOT report `UserData.IsFavorite` until a file-keyed favourite exists in the SDK (CORE-2895).

#### Scenario: Favourite-only film

- **WHEN** a user marked a film favourite and never played or rated it
- **THEN** no record is reported for that user and film

### Requirement: Jellyfin collections

When collections are selected, the Jellyfin source SHALL report each file in a BoxSet (a `BaseItems` row of type `MediaBrowser.Controller.Entities.Movies.BoxSet`) as a collection member named for the BoxSet, at its `LinkedChildren.SortOrder` position. Members that are folders or have no path SHALL be skipped. Collection members SHALL be reported under every source user.

#### Scenario: BoxSet members

- **WHEN** BoxSet "Heist Night" links Heat, Arrival and an episode in that order
- **THEN** each user's records include the three files as members of "Heist Night" at positions 0, 1 and 2
