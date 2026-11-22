## ADDED Requirements

### Requirement: The plugin reads the full Kodi NFO family

The plugin SHALL read a media file's own `<stem>.nfo` (one film root, or one or more episode roots for
a multi-episode file), the folder film NFO when there is no per-file NFO, the nearest `season.nfo`
and the nearest `tvshow.nfo`. It SHALL read title, original title, sort title, plot, outline,
tagline, ratings with votes, user rating, certification, genres, studios, tags, set, premiered or
aired date, runtime, directors, writers, actors (name, role, order), trailer and date added.

#### Scenario: A Kodi film NFO

- **WHEN** a film file has a `<stem>.nfo` with a plot, a sort title, two genres and three actors
- **THEN** the sidecar plan carries the plot as the summary, the sort title, both genres and the
  three credits with their roles and billing order

#### Scenario: A multi-episode file

- **WHEN** a file's NFO holds two episode roots
- **THEN** the plan's title joins both episode titles and its summary joins both plots

### Requirement: The NFO reader keeps what parsed

The reader SHALL keep every element it decoded before a malformed or truncated point, and SHALL
decode an ISO-8859-1 or Windows-1252 file according to its XML encoding declaration.

#### Scenario: A truncated NFO

- **WHEN** an NFO ends in the middle of its actor list
- **THEN** the title, plot and the actors before the cut are still in the plan

#### Scenario: A Latin-1 NFO

- **WHEN** an NFO declares `encoding="ISO-8859-1"` and its title holds Latin-1 accented bytes
- **THEN** the plan's title holds the same characters as UTF-8

### Requirement: The sidecar signature tracks every NFO read

`NFOSignature` SHALL change when any NFO that feeds a file's plan appears, is removed, or changes
size or modification time, and SHALL be empty when the file has no NFO.

#### Scenario: Editing the NFO

- **WHEN** a film's NFO is rewritten with a longer plot
- **THEN** the file's signature differs from the one before the edit

### Requirement: The NFO migration source carries watch state

The `nfo` migration source SHALL emit watch state for a file whose NFO has `playcount` above zero
or `watched` true (watched) and SHALL carry `lastplayed` as the play instant. It SHALL declare the
watch-state category.

#### Scenario: playcount 1

- **WHEN** a film NFO has `<playcount>1</playcount>` and `<lastplayed>2024-03-01 20:15:00</lastplayed>`
- **THEN** the source item for that file carries a present, watched watch state at that instant
