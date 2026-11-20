# Close the year picker's remaining edge cases

Tickets: CORE-2721, CORE-2717 (`queue:core:main`).
Status: proposed.

## Why

CORE-2701 left known gaps in the year picker, recorded in CORE-2721, and CORE-2717 tracks the
dashed-year episode forms it does not read:

- An underscore never bounds a year: `Heat_1995_1080p` parses as `Heat 1995` with no year,
  `2012_2009_1080p` as `2012 2009`, and `Blade.Runner.2049.2017_1080p` as `Blade Runner`, 2049.
- A series named with a year loses its absolute episodes when spaced or dotted:
  `Space 1999/Season 01/Space 1999 0001` parses as the film `Space`, 1999.
- Release-group brackets joined by a separator (`[Fansub].[Group (1999)]`) expose the year inside
  the second bracket, and a film keeps its leading group in the title (`[Group] 2012 (2009)`).
- A checksum bracket can become the whole title (`[CAFEBABE] (2007) [DEADBEEF]`), and a scene tag
  before a checksum becomes the title (`[REC] (2007) PROPER.[DEADBEEF]` parses as `PROPER`).
- `Show - 2011 - 12`, `Show_-_2011` and `Show_-_2011_-_12` read no episode or no year.

## What Changes

- An underscore bounds a year like a space or a dot.
- A bare year directly before a four-digit absolute number is part of the series name, given the
  episode context the number already needs.
- Leading release-group brackets may be separated by spaces, dots, underscores or hyphens.
- A film's title drops its leading release-group brackets when title text remains after them.
- A checksum bracket is never title text.
- After a leading parenthesized year, a scene tag (`PROPER`, `REPACK`, `RERIP`) followed only by a
  checksum is release noise; other release words there stay the title (`DV [DEADBEEF]`).
- A dashed number in the year range is skipped as the series' year, and a later dashed number is
  read as the absolute episode (`Show - 2011 - 12`).

## Impact

No SDK change. Names in the forms above change title, year or episode. `Show - 2015`,
`Heat (1995) - 1080`, `Hunter x Hunter (2011) - 12`, `Space_1999_0001` and the CORE-2701 cases
parse as before.
