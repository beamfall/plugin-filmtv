# interop-declaration Specification

## Purpose
The interop tokens Film & TV declares so Core's compatibility surfaces can show external automation
tools its movies, episodes, shows, seasons and scoped libraries.

## Requirements

### Requirement: Film & TV declares its interop tokens

The bundle SHALL map:
- class `movie` to `Movie` and class `episode` to `Episode`;
- kind `show` to `Series` and kind `season` to `Season`;
- scope `films` to `movies` and scope `tvshows` to `tvshows`.

It SHALL leave every other class value, the `film-series` kind and the `mixed` scope unmapped.

#### Scenario: Declared tokens

- **WHEN** Core reads `Bundle().Interop`
- **THEN** it finds exactly these mappings, and `ValidateBundle` accepts the bundle

#### Scenario: Mixed library

- **WHEN** a library uses the default `mixed` scope
- **THEN** no collection token is declared for it, and its items still carry per-item types
