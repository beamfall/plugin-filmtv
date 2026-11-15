## MODIFIED Requirements

### Requirement: Shows grid on the library home

The library home Layout SHALL include the show Entities as a shelf (one card per show), placed
right after Next Up as the `browse-shows` Section described in the browse-layout spec. See All on
that shelf lists every show.

#### Scenario: Home layout

- **WHEN** the home Layout is read
- **THEN** it has a shelf Section whose Source is `entity-items` over the `show` Kind
