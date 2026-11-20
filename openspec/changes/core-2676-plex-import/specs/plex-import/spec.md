## ADDED Requirements

### Requirement: Plex account state

The Plex source SHALL key playback state and ratings by source account using the account name and ID in the form `name#id`. It SHALL resolve settings through the metadata GUID and expose each matching media file. A rating SHALL preserve its value on a scale of ten, including zero. A NULL rating SHALL be absent.

#### Scenario: Two Plex accounts have settings for the same file

- **WHEN** the same metadata GUID has settings for two known accounts
- **THEN** each account's playback state and rating appear under its own source-user key
- **AND** settings without a GUID do not supply playback state or a rating

### Requirement: Plex collection memberships

The Plex source SHALL advertise watch-state, ratings and collections. It SHALL emit each collection-tag membership as its own file-keyed record, preserving the name and position. Collection memberships are library-wide and are available to each named source account independently of its playback state.

#### Scenario: A file belongs to two collections

- **WHEN** the file has two collection taggings
- **THEN** the source emits a membership for each collection with its own position
- **AND** a tagging of another type does not create a collection membership
