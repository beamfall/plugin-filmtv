## ADDED Requirements

### Requirement: Identified episode name in the display title

The parser SHALL implement `IdentifiedMediaTitle(path, probe, fields)`, the display title Core
writes when an identification is applied, given the identification's plugin-declared Media field
values by name. For an episode whose `fields` carry a non-blank `episode-title`, it SHALL return the
episode code, ` · ` and that title with its whitespace collapsed; otherwise it SHALL return what
`MediaTitle` returns for the path.

#### Scenario: Identified name replaces the filename's

- **WHEN** the path is `Ted.Lasso.S03E02.I.Dont.Want.to.Go.to.Chelsea.2160p.WEB-DL.mkv` and
  `episode-title` is `I Don't Want to Go to Chelsea`
- **THEN** `IdentifiedMediaTitle` returns `S03E02 · I Don't Want to Go to Chelsea`

#### Scenario: Identified name for a code-only file

- **WHEN** the path is `Ted.Lasso.S03E01.2160p.WEB.H265-GGEZ.mkv` and `episode-title` is
  `Smells Like Mean Spirit`
- **THEN** `IdentifiedMediaTitle` returns `S03E01 · Smells Like Mean Spirit`

#### Scenario: No identified name

- **WHEN** `fields` carry no `episode-title`
- **THEN** `IdentifiedMediaTitle` returns the path's `MediaTitle`
