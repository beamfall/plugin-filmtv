# Give Core what it needs to group local episode files into shows and seasons

Ticket: CORE-1411 (`queue:core:main`). Status: proposed.

## Why

A library of local episode files with no metadata provider shows a flat list of episodes. Core
creates show and season containers only when a provider applies metadata, and every episode card
carries the series name as its title, so four episodes of one series look identical. The parser
already reads the series name, the season and episode numbers and the episode's own title from the
filename (CORE-1148), but the bundle gives Core nowhere to put them: the season Kind declares no
relations, and nothing says which parsed value is the title to show.

## What Changes

- The season Kind declares two relations: `season-episode` to Media (cardinality one, so an episode
  belongs to one season) and `season-of` to the show Kind.
- A new flag Media field `filmtv-hydrated` marks a file whose local containers were written, so a
  rescan does not write them again.
- `LocalHydration(path)` returns the local grouping for a path: whether it is an episode, the series
  title, the season number, the season title (`Season N`, or `Specials` for season 0), the episode
  index range and the episode title. It reads the path only and agrees with `Parse`.
- The parser implements Core's optional media-title hook, `MediaTitle(path, probe)`: an episode with
  an episode title in its name shows that title; everything else keeps the parsed title. The parsed
  title stays the series name, because it is the provider search query.
- The library home gets a "Shows" grid over show Entities.

## Non-goals

- Writing the containers. Core's hydration mapper does that from `LocalHydration`, in the Core half
  of CORE-1411.
- Provider-side show and season Entities. The TMDB and TheTVDB scrapers identify Media directly and
  create no show or season Entities, so local containers are never duplicated by a later match.

## Failure modes

- A file whose name has no series and no season folder above it gets no containers, the same files
  the parser already refuses.
- Two different series with the same name in one library share one show container.
