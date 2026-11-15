# Pick trailers to play before a film

Ticket: CORE-1701 (`queue:core:main`). Status: proposed.

## Why

Cinema mode plays an Owner's pre-roll clips and a few trailers before a film, the way a theatre
does. Core owns a generic pre-play queue (the Profile's settings, the clip list, visibility and
playback) but cannot know what a film or a trailer is. The Film & TV plugin decides when a feature
qualifies and which trailers to play.

## What Changes

- `PlanPrePlay`, the plugin policy Core calls through an adapter before it starts a feature:
  - Only a feature whose `class` field is `movie` is eligible. Anything else gets no pre-play.
  - Trailers are Extras whose filename stem ends in `-trailer` or whose nearest extras folder is
    named `trailers`, linked to a parent whose `class` is `movie`.
  - The feature's own trailers are skipped, so the film is not previewed before itself.
  - At most one trailer per parent film. Films the Profile has not finished come first, then the
    rest. Within each group the order is a seeded shuffle, so the same seed gives the same picks.
  - The request asks for 0 to 10 trailers; a larger count is capped at 10.
- `PrePlayReader`, the store view Core implements: Media class values, the Extras in the feature's
  Library and the Profile's watched set. Every read honours the caller's Library visibility.

## Non-goals

- Owner pre-roll clips and the Profile setting: Core stores and plays them (CORE-1701, core).
- Trailers fetched from a provider. CORE-0363 covers trailers that are not in the catalog.
- Client playback and the skip control.

## Failure modes

- A reader error fails the plan; Core then plays the feature with no pre-play.
- A feature with no visible class, or a class other than `movie`, is not eligible.
- A Library with no trailers yields an eligible plan with no trailers, so Core still plays clips.

## Acceptance evidence

- Unit tests for eligibility, trailer detection, own-trailer exclusion, one-per-parent, unwatched
  first, seeded determinism, the count cap and reader errors.
- `script/gate.sh`.

## Rollback

Revert the change. Core falls back to clips only when no policy is registered.
