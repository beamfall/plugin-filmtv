## ADDED Requirements

### Requirement: A release word before the year is not title text

The parser SHALL end a film title before a release word that would end the title in front of a
resolution, when the year follows it, and SHALL keep a word that ends a title only in front of a
release tag.

#### Scenario: Edition word before the year

- **WHEN** the parser reads `Jurassic.World.Dominion.EXTENDED.2022.2160p.UHD.BluRay.REMUX.mkv`
- **THEN** the title is `Jurassic World Dominion` and the year is 2022

#### Scenario: Real titles holding release words

- **WHEN** the parser reads `Extended.Family.2023.1080p.mkv`, `Internal.Affairs.(2002).mkv` or `Charlotte's.Web.2006.mkv`
- **THEN** the titles are `Extended Family`, `Internal Affairs` and `Charlotte's Web`

### Requirement: Language and subtitle tags are release noise

The parser SHALL treat French and Scandinavian language and subtitle tags as release noise, and
SHALL end a title at `DC` when a release tag follows it.

#### Scenario: Edition and language tags without a year

- **WHEN** the parser reads `Color.Of.Night.Unrated.DC.VostFR.BRrip.x264.mkv`
- **THEN** the title is `Color Of Night`

### Requirement: A leading site name is not title text

The parser SHALL drop a leading `www.` site name followed by a spaced hyphen, or one in brackets,
before reading the title.

#### Scenario: Site prefix

- **WHEN** the parser reads `www.Torrenting.com - Anatomy Of A Fall (2023).mkv` or `[www.Torrenting.com] Anatomy Of A Fall (2023).mkv`
- **THEN** the title is `Anatomy Of A Fall` and the year is 2023
