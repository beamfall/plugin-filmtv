# Interop declaration

Ticket: CORE-2812 (`queue:core:main`), part of CORE-2681. Depends on plugin-sdk `Bundle.Interop`
(CORE-2810, SDK v0.4.6).

## Why

Core's read-only compatibility surfaces for external automation tools need an item type for each
listed item and a collection token for each library. Core never spells Film & TV vocabulary, so this
plugin declares both as data.

## What Changes

- `Bundle().Interop` declares these tokens:
  - class `movie` → `Movie`, class `episode` → `Episode`
  - kind `show` → `Series`, kind `season` → `Season`
  - scope `films` → `movies`, scope `tvshows` → `tvshows`
- `film-series`, any unknown class and the default `mixed` scope stay unmapped.
- Requires plugin-sdk v0.4.6. `SDKFloor` and `SDKCeiling` move to `0.4.6`.

## Non-goals

- No streaming, playback or watch-state declaration.
- No change to parsing, matching or scrobbling.

## Acceptance evidence

- `TestInteropDeclaration` pins the declaration.
- `ValidateBundle` accepts the bundle.
- `script/gate.sh` passes.

## Rollback

Revert the commit and release a patch. Core lists nothing for this plugin until the declaration
returns.
