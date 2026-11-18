# TMDB cast characters, billing and key crew

Tickets: CORE-2503 and CORE-2432 (core `queue:core:main`). Core change: `core-2503-cast-crew` in
beamfall; SDK change: `entity-map-relation-attributes` in plugin-sdk.

## Why

The `credited-in` relation declares `character`, `billing`, `department` and `role`, but the
`credits.cast` EntityMaps write none of them, and no map reads `credits.crew`. Clients cannot show
who an actor plays, the billing order, or the director and writers.

## What changes

- The `credits.cast` maps on `tmdb-movie` and `tmdb-tv` write `character` and `billing` (TMDB
  `order`) on the relation row and order credits by billing.
- A second EntityMap on `credits.crew` imports the key jobs (Director, Screenplay, Writer, Producer,
  Original Music Composer, Director of Photography) with `job` and `department`, after the cast.
- `credited-in` declares `job`.

Needs the plugin-sdk release carrying `RelationAttributeMap`, `ElementFilter` and `OrdinalOffset`.

## Non-goals

No person lookups for crew (crew elements carry the name and headshot; cast lookups keep their cap).
No other crew jobs. No change to the show-level `credit` relation.

## Failure modes

A crew member holding several key jobs is listed once per job by TMDB; core merges them into one
credit with the jobs joined. A title with no key crew writes cast only.
