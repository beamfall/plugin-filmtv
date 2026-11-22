# nfo-import Specification

## Purpose
How the Film & TV plugin reads Kodi-family NFO sidecars beside media files: which values, relations and watch state it imports, how it tells a missing sidecar from one it could not read, and how the sidecar signature drives re-import.

## Requirements

### Requirement: The plugin reads the Kodi NFO of each media file

The plugin SHALL read a media file's own `<stem>.nfo` (one film root, or one or more episode roots
for a multi-episode file), else the folder film NFO when the file is the folder's main feature, with
every metadata element a Kodi film or episode NFO defines (titles, plot, ratings, certification,
genres, studios, tags, set or collection, dates, runtime, credits, trailer). Container sidecars
(`tvshow.nfo`, `season.nfo`) are not read for hydration.

#### Scenario: A Kodi film NFO

- **WHEN** a film file has a `<stem>.nfo` with a plot, a sort title, two genres and three actors
- **THEN** the sidecar plan carries the plot as the summary, the sort title, both genres and the
  three credits with their roles and billing order

#### Scenario: A multi-episode file

- **WHEN** a file's NFO holds two episode roots
- **THEN** the plan's title joins both episode titles and its summary joins both plots

#### Scenario: An episode with a year and no air date

- **WHEN** an episode NFO has `<year>1999</year>`, `<sorttitle>First</sorttitle>` and no air date
- **THEN** the plan's year is 1999 and its sort title is "First"

#### Scenario: A trailer beside a folder film NFO

- **WHEN** a folder holds `Film.mkv`, `Film-trailer.mkv` and a folder film NFO
- **THEN** the feature's plan carries the NFO and the trailer's plan is empty

#### Scenario: A sample named with a space

- **WHEN** a folder holds `Film.mkv`, `Film sample.mkv` and a folder film NFO
- **THEN** the sample's plan is empty: a trailer or sample suffix may follow a hyphen, dot,
  underscore or space, and the other extra suffixes a hyphen

#### Scenario: Legacy multi-person credits

- **WHEN** an NFO has `<director>A / B</director>` or `<credits>A / B</credits>`
- **THEN** the plan carries one credit for A and one for B, not one for "A / B"

#### Scenario: Tags in a later episode root

- **WHEN** a two-episode NFO names a tag only in its second episode root
- **THEN** the plan's tags include it

#### Scenario: A non-finite rating

- **WHEN** an NFO's rating value is `NaN` or `Inf`
- **THEN** the plan carries no rating and keeps its other values

### Requirement: The NFO reader keeps what parsed

The reader SHALL keep every element it decoded before a malformed or truncated point, and SHALL
decode an ISO-8859-1 or Windows-1252 file according to its XML encoding declaration.

#### Scenario: A truncated NFO

- **WHEN** an NFO ends in the middle of its actor list
- **THEN** the title, plot and the actors before the cut are still in the plan

#### Scenario: A Latin-1 NFO

- **WHEN** an NFO declares `encoding="ISO-8859-1"` and its title holds Latin-1 accented bytes
- **THEN** the plan's title holds the same characters as UTF-8

### Requirement: A failed read is distinct from no sidecar

The plan SHALL be marked unreadable when the sidecar exists but cannot be opened or read, when it
is larger than the read limit, when it changes while it is read, or when it holds an XML syntax error
before any root element. An
unreadable plan carries no values and no signature. A sidecar whose text holds no XML element at all
(a bare provider URL) is a readable sidecar with no metadata.

#### Scenario: Permission denied

- **WHEN** a media file's NFO exists but cannot be opened
- **THEN** the plan is unreadable, not empty

#### Scenario: An oversized NFO

- **WHEN** a well-formed NFO is larger than the read limit
- **THEN** the plan is unreadable, so the credits past the limit are not dropped

### Requirement: Only a clean not-found means no sidecar

Looking up a candidate sidecar that fails for any reason other than the file not existing SHALL
make the plan unreadable, and `NFOSignature` SHALL return an error rather than an empty signature.

#### Scenario: The sidecar's folder denies traversal

- **WHEN** looking up a media file's NFO fails with a permission error
- **THEN** the plan is unreadable and `NFOSignature` returns an error

### Requirement: The sidecar signature tracks the NFO read

The plan SHALL carry the signature of the bytes it parsed: the NFO's absolute path, its size and
its modification time. `NFOSignature` SHALL compute the same value from
the file system without reading the file, SHALL change when the NFO appears, is removed, or changes
name, size or modification time, and SHALL be empty when the file has no NFO.

#### Scenario: Editing the NFO

- **WHEN** a film's NFO is rewritten with a longer plot
- **THEN** the file's signature differs from the one before the edit, and the next plan carries the
  new signature

### Requirement: The plugin names the relations an NFO owns

`NFOOwnedRelations` SHALL list every Entity kind and relation name a plan can carry (genre,
studio, credit and film-series membership), so Core can remove the ones a later NFO no longer
names.

#### Scenario: The last actor is removed

- **WHEN** an NFO that named one actor is rewritten with none
- **THEN** the plan carries no credit relation and the owned relations still name the credit
  relation

### Requirement: Only a US or unqualified rating is the policy certification

The plan SHALL take the US entry of a country-qualified rating list, else the first unqualified
entry, as the policy certification when it is a value of the certification scale. A qualifier is
any country code or name before a colon. A rating that names only other countries SHALL be display
text only.

#### Scenario: A Canadian and a US rating

- **WHEN** an NFO has `<mpaa>CA:PG / US:R</mpaa>`
- **THEN** the plan's certification is R

#### Scenario: A foreign-only rating

- **WHEN** an NFO has `<mpaa>CA:PG</mpaa>`
- **THEN** the plan carries PG as display text and no certification

#### Scenario: A country name before the unqualified rating

- **WHEN** an NFO has `<mpaa>Germany:FSK 16 / R</mpaa>`
- **THEN** the plan's certification is R

### Requirement: The NFO migration source carries watch state

The `nfo` migration source SHALL emit watch state for a file whose NFO has `playcount` above zero
or `watched` true (watched) and SHALL carry `lastplayed` as the play instant, only when the run
selected the watch-state category. A folder film NFO's play SHALL apply to the main feature only.

#### Scenario: playcount 1

- **WHEN** a film NFO has `<playcount>1</playcount>` and `<lastplayed>2024-03-01 20:15:00</lastplayed>`
- **THEN** the source item for that file carries a present, watched watch state at that instant

#### Scenario: A collections-only run

- **WHEN** an NFO import run selects only the collections category
- **THEN** no source item carries watch state

### Requirement: An unreadable NFO fails the watch-state source

An NFO that exists but cannot be read SHALL fail the `nfo` source with an error rather than emit
its file without watch state, so a resumed run does not skip it. Resuming SHALL NOT read the NFOs
of items before the checkpoint.

#### Scenario: An unreadable NFO during a watch-state run

- **WHEN** a film's NFO records a play but cannot be opened
- **THEN** the source returns an error instead of emitting the film without watch state
