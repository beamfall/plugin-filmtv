## ADDED Requirements

### Requirement: Movie filenames carry a version label

The filename parser SHALL return a `VersionLabel` for a non-episode filename that has a
parenthesized year followed by whitespace, a hyphen, whitespace and a label, and SHALL return the
same title, class and year as the name without the label.

#### Scenario: Resolution versions

- **WHEN** a directory holds `Title (2004) - 1080p.mkv` and `Title (2004) - 2160p.mkv`
- **THEN** both parse to title `Title`, class `movie`, year 2004, with labels `1080p` and `2160p`

#### Scenario: Bracketed edition

- **WHEN** a file is named `Title (2004) - [Extended Edition].mkv`
- **THEN** its label is `Extended Edition`

### Requirement: Non-version suffixes carry no label

The parser SHALL return an empty `VersionLabel` for dotted release names, names without a
parenthesized year, episodes, stacked parts and extras.

#### Scenario: Stacked part or extra

- **WHEN** a file is named `Title (2004) - cd1.mkv` or `Title (2004) - trailer.mkv`
- **THEN** its label is empty

#### Scenario: Release name

- **WHEN** a file is named `Title.2004.1080p.BluRay.x264.mkv`
- **THEN** its label is empty
