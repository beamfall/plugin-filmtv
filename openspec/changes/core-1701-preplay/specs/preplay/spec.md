## ADDED Requirements

### Requirement: Pre-play eligibility

The plugin SHALL report a feature as eligible for pre-play only when its `class` field is `movie`.
A feature with any other class, or with no visible class, SHALL NOT be eligible and SHALL get no
trailers.

#### Scenario: Film is eligible

- **WHEN** the feature's class is `movie`
- **THEN** the plan is eligible

#### Scenario: Other media is not

- **WHEN** the feature's class is `episode`
- **THEN** the plan is not eligible and holds no trailers

### Requirement: Trailer selection

The plugin SHALL pick trailers from the Extras of the feature's Library. An Extra SHALL count as a
trailer when its filename stem ends in `-trailer`, case-insensitively, or when its nearest
ancestor folder with an extras name is named `trailers`. Its parent SHALL have class `movie` and
SHALL NOT be the feature. The plan SHALL hold at most one trailer per parent and at most the
requested count, capped at 10.

#### Scenario: Own trailer skipped

- **WHEN** the only trailer in the Library belongs to the feature
- **THEN** the plan is eligible and holds no trailers

#### Scenario: Featurette is not a trailer

- **WHEN** an Extra is named `Title-featurette.mkv` in a `featurettes` folder
- **THEN** it is not picked

### Requirement: Trailer order

The plugin SHALL order trailers for parents the Profile has not finished before trailers for
parents it has finished. Within each group the order SHALL be a shuffle seeded by the request seed,
so equal seeds and equal inputs give equal plans.

#### Scenario: Unwatched first

- **WHEN** the Profile finished film A but not film B, and one trailer is requested
- **THEN** the plan holds B's trailer
