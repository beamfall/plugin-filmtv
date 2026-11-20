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
  the PTT version and commit they came from, and PTT's licence (`testdata/ptt/LICENSE`).
- `testdata/ptt/import.py` regenerates the file from a PTT checkout without running PTT. It keeps
  a case unless its fixtures mark it complete, adult or trash, or declare more than one season, a
  season with no episode, or more than three episodes. The rule is written in the file, so packs
  are dropped by rule, not by picking failures.
- A test grades title, year, season and episode on the set. Each field that does not parse today
  is marked in `testdata/ptt/unresolved.json`, written by the test's `-update-ptt` flag. A
  resolved field that regresses fails, a marked field that starts passing fails until the marks
  are rewritten, and each field's rate has a documented floor at or below the current rate.

## Impact

Test data and a test only; no parser change. PTT's names are graded as written, with `.mkv`
appended when they carry no video extension.
