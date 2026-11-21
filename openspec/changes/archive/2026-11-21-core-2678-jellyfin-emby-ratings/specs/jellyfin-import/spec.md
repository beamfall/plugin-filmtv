## ADDED Requirements

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
