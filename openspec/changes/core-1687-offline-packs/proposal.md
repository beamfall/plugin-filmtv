# Resolve season, series and next-unwatched offline packs

Ticket: CORE-1687 (`queue:core:main`). Status: proposed.

## Why

Core's offline pack service downloads a frozen, encrypted set of Media for offline play, but only
the music plugin tells it what a pack holds (album, artist, playlist). A viewer cannot take a season
or a whole series on a flight, and cannot keep the next few episodes of a show they are watching on
the device. Jellyfin clients queue season and series downloads; Beamfall should match that and add
an automatic "up next" pack.

## What Changes

- Three offline pack target kinds, resolved by the plugin and registered into Core by an adapter:
  - `filmtv-season`: one season's episodes in stored order. The ref is the season Entity ID.
  - `filmtv-series`: every season of a show, numbered seasons ascending, then unnumbered seasons,
    then specials (flagged `specials` or numbered 0). The ref is the show Entity ID.
  - `filmtv-next-unwatched`: up to N episodes after the last one the requesting Profile finished,
    skipping specials and any episode finished later. The ref is `<show Entity ID>:<N>`, N from 1
    to 50.
- `PackMediaReader`, the store view Core implements: members of a container, the Entities pointing
  at a parent through a relation (with their number and flag fields) and the Profile's watched set.
  Every read is filtered by the caller's Library visibility.

## Non-goals

- Building, transcoding or encrypting packs. Core owns that, including the download quality choice.
- Client UI for queueing packs.
- Provider-side show and season Entities: seasons come from local containers (CORE-1411).

## Failure modes

- An invisible or missing season or show fails the resolve with the reader's error.
- A show with no visible seasons resolves to an empty pack, which Core refuses.
- A malformed next-unwatched ref fails before any read.
