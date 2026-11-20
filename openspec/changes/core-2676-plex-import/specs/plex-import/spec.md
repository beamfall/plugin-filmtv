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

### Requirement: Plex category selection

The scoped Plex source SHALL limit imported items to the selected categories. Its compatibility item reader and stream SHALL emit watch state only when watch state is selected. The per-account reader SHALL emit items in the selected categories.

#### Scenario: The caller supplies a nil selection

- **WHEN** the scoped Plex source is constructed with a nil category selection
- **THEN** the source selects watch state by default

#### Scenario: The caller explicitly selects no categories

- **WHEN** the scoped Plex source is constructed with a non-nil empty category selection
- **THEN** the source selects no categories and emits no items through any reader
