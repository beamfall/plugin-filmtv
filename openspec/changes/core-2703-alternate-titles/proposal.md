# Dual-title names search by their Latin title

Ticket: CORE-2703 (`queue:core:main`).
Status: proposed.

## Why

Russian, Chinese and Japanese releases often carry two titles: a native one and a Latin one,
separated by a bar or with the Latin title in parentheses after the native one. The whole pair
becomes the title (`Голубая волна | Blue Crush (2002)` parses as `Голубая волна | Blue Crush`,
`О мышах и людях (Of Mice and Men) 1992` as `О мышах и людях (Of Mice and Men)`), so the provider
search query matches nothing well. A filename cannot hold `/`, so on disk the slash in
`Голубая волна / Blue Crush` is written as `|` or as the full-width `／` or `｜`.

## What Changes

- A title made of a Latin-script title and a title in another script, split by `|`, `／` or `｜`,
  or with one in parentheses at the end of the other, parses to the Latin title.
- The other title is kept in a new declared, searchable text Media field `alternate-title`, so the
  native title still finds the item in library search.
- Titles that are both Latin (`Star Wars (Special Edition)`) or have no Latin title (`Жихарка`)
  keep their whole title.

## Impact

No SDK change. Only names of the forms above change. Using the alternate title as a fallback
identification query needs an SDK seam and a core change: CORE-2746.
