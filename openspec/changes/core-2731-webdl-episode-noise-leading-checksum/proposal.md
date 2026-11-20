# WEB-DL scene noise, episode scene noise and leading checksums

Ticket: CORE-2731 (`queue:core:main`).
Status: proposed.

## Why

CORE-2726's Codex review left four parser gaps:

- A trailing `WEB-DL` reads as `WEB` plus a release group `DL`, so scene noise ending in it stays a
  title (`[REC] (2007) PROPER.REPACK.WEB-DL` parses as `PROPER REPACK WEB-DL`).
- After a bare leading year, scene noise before an episode marker becomes the series name
  (`/tv/Fringe/Season 01/(2007) PROPER.1080p.S01E01 - Pilot.mkv` parses as series `PROPER 1080p`).
- A leading checksum counts as a release-group bracket, so the year-named title after it becomes
  the year and the name does not parse (`[DEADBEEF] 1923`).
- Whether a checksum cut should still strip a trailing release word was undecided:
  `Movie Uncut [DEADBEEF]` parses as `Movie Uncut`, `Movie Uncut` as `Movie`.

## What Changes

- A release tag split by a hyphen (`WEB-DL`) is one tag, not a tag and a release group.
- An episode whose name is only scene noise after a bare leading year takes its series name from
  the show folder.
- A checksum bracket at the start of a name is not a release group: a year-named title after it is
  the title, not the year, and the checksum does not end the title.
- Decision: a checksum keeps a release word before it in the title. It ends the title the way a
  year does, so `Movie Uncut [DEADBEEF]` stays `Movie Uncut`, matching the pinned CORE-2726 case
  `[Group] (2013) Proper Uncut [DEADBEEF]` → `Proper Uncut`.

## Impact

No SDK change. Only names of the forms above change. Every case pinned by CORE-2701, CORE-2721
and CORE-2726 parses as before, and `[Group] 1923` keeps its release-group handling.
