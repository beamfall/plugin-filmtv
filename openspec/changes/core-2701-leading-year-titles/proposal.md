# Read a year that leads a film name as title text, or as the year when parenthesized

Ticket: CORE-2701 (`queue:core:main`).
Status: proposed.

## Why

The year picker took the first standalone year as the release year and cut the title there. When a
name starts with a year, the title before it is empty and the parse fails: `2012.2009.1080p.BluRay`,
`2019 After The Fall Of New York 1983 REMASTERED`, `2001.A.Space.Odyssey.1968.1080p`,
`1917.2019.1080p`, `/tv/1923.S01E01` and `2012.mkv` all return no title, so the file drops out of
identification. A leading parenthesized year (`(2000) The Emperor's New Groove`) fails the same way.
A title ending in a number followed by the year in dotted form (`Blade.Runner.2049.2017`) parses as
`Blade Runner`, 2049.

## What Changes

- A standalone year with no title text before it is title text, not the year.
- A standalone year directly followed by another year (only separators between) is title text; the
  later one is the year.
- A parenthesized year after title text stays preferred, as before (`1917 (2019)`).
- A parenthesized year with no title text before it (a leading release-group bracket aside) is the
  year, and the title is the text after it.
- Show-folder and absolute-episode series names use the same reading, so a series named after a
  year (`1923`) keeps its name.

## Impact

No SDK change. Names that returned no parse now parse; `Blade.Runner.2049.2017` changes title and
year. Names whose first year already had title text before it, and was not followed by another year,
parse as before.
