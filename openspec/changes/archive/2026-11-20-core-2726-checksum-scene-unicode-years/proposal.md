# Checksum folders, lone scene tags and non-ASCII year boundaries

Ticket: CORE-2726 (`queue:core:main`).
Status: proposed.

## Why

CORE-2721 left four parser gaps, found in its Codex review:

- A show folder named only by a checksum becomes the series name
  (`/tv/[DEADBEEF]/Season 01/S01E01.mkv` parses as series `[DEADBEEF]`).
- A scene tag after a leading year, with nothing before that year, becomes the film title
  (`(2007) PROPER.[DEADBEEF]` parses as `PROPER`).
- A release tag after the checksum stops the scene-tag rule
  (`[REC] (2007) PROPER.[DEADBEEF].1080p` parses as `PROPER`).
- The year boundary is ASCII-only: `Café1999` reads as `Café` plus the year 1999, so
  `Show (2011) Café1999 0001` parses as an episode of `Show (2011) Café1999`.

## What Changes

- A checksum bracket is never a series name taken from a folder.
- After a leading parenthesized year, a scene tag that only checksums and release tags follow is
  release noise. With nothing before the year, the name has no title and does not parse.
- A release word that can be a title (`Uncut`) still keeps the scene tag as the title
  (`Proper Uncut`).
- A letter, digit or combining mark of any script next to a four-digit number means the number is
  not a year.

## Impact

No SDK change. Only names of the forms above change. Every case pinned by CORE-2701 and CORE-2721
parses as before, including `Café1999 0001` (episode 1) and `[Group] (2013) DV [DEADBEEF]` (`DV`).
