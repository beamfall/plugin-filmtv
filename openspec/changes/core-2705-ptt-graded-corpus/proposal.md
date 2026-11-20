# Grade the parser on PTT's library-shaped fixtures

Ticket: CORE-2705 (`queue:core:main`).
Status: proposed.

## Why

The parse corpus holds 52 hand-written cases, so a parser gap only shows once someone writes a
case for it. PTT (dreulavelle/PTT, MIT) ships several hundred release names with the title,
year, season and episodes its maintainers expect. Grading the parser on the ones that look like
library files gives a second, independent rate, and the parser gap tickets then show up as that
rate rising.

## What Changes

- `testdata/ptt/cases.json` carries PTT's library-shaped fixtures with the fields they declare,
  the PTT version and commit they came from, and PTT's copyright and MIT notice, also beside it
  in `testdata/ptt/LICENSE`.
- `testdata/ptt/import.py` regenerates the file from a clean PTT checkout without running PTT.
  It keeps a case unless its fixtures (including PTT's collection, adult, trash and season-list
  tests, read only for that) mark it complete, adult or trash, or declare more than one season,
  more than three episodes, a season with an empty episode list, or a season with no episode list
  on a name without a video extension. The rule is written in the file, so packs are dropped by
  rule, not by picking failures.
- A test grades title, year, season and episode on the set. Each field that does not parse today
  is marked in `testdata/ptt/unresolved.json`, written by the test's `-update-ptt` flag. A
  resolved field that regresses fails, a marked field that starts passing fails until the marks
  are rewritten, and each field's rate has a documented floor at or below the current rate. A name the parser rejects gets every
  declared field wrong.

- Two misreads the set found are fixed: `S01E1056` was cut to episode 105 and is now the absolute
  episode 1056, as an episode above 999 after season and episode words already is (a digit after
  four, as in `S01E1231080p`, still leaves episode 123); and a channel layout (`AC3 5.1x2+2.0x3`)
  was read as season 1, episode 2. A layout needs an audio codec before it or a `+` joining it to
  another, so a title's number (`Babylon.5.1x02`, `Doctor.Who.2005.8x11`) still leaves a season
  code.

## Impact

No SDK change. The other misses are marked, not fixed: their parser gaps have their own tickets. PTT's names are graded as written, with `.mkv`
appended when they carry no video extension.
