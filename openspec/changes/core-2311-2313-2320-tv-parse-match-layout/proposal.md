# TV episode parsing, series matching and class-neutral home rows

Tickets: CORE-2311 (T07), CORE-2313 (T09), CORE-2320 (T16) (`queue:core:main`).
Status: proposed.

## Why

A tower QA pass on a real TV library found episodes filed as movies with raw file names as titles
(`1e01-Yesterday's Jam`, `3 01 From Hell`, `Dealbreakers Talk Show 0001`), every matched episode of
a show titled with the show name and dated with the show's premiere year, scene names without
apostrophes (`A Mothers Son`) failing to match the provider's `A Mother's Son`, TheTVDB shown as a
required key, and a Films library showing rows titled "New Movies & Episodes" and
"All Movies & Episodes".

The tower's unmatched queue showed why TV matched nothing (CORE-2311): about 1215 `tmdb-tv` rows
were `below-threshold` with the right candidate. An episode name has no year, so any non-exact
title lost 0.25, and every show-level candidate lost 0.05 more for carrying no episode numbers:
`The Traitors UK` against `The Traitors` scored 0.55, `Key and Peele` against `Key & Peele` 0.367.

## What Changes

- The filename parser reads three more episode forms, each only with episode context (a season
  folder, a show folder of the series' name, a TV or anime folder, or an anime group or checksum
  bracket), and never for a camera clip stem such as `VID_0001`:
  - a season-by-episode code with a bare `e` (`1e01-Yesterday's Jam`), never after a channel layout
    (`DD5.1e01`);
  - a leading `season episode` number pair (`3 01 From Hell`), only inside a season folder whose
    number agrees, or in `Specials` as season 0;
  - an absolute episode number zero-padded to four digits (`Dealbreakers Talk Show 0001`), or to
    three after ` - ` (`Naruto - 001`), ending the title or before ` - <episode title>`, when the
    name carries no other episode token and no year. It carries an index but no season; the local
    grouping files it under its season folder, else Season 1, and its display code is `E01`.
- A CRC32 checksum bracket (`[ABCD1234]`) is release noise. Image files are refused.
- The match scorer removes apostrophes (`'`, `’`, `‘`, `ʼ`, backtick), joins a lone `s` to the word
  before it (`Ocean_s_Eleven`), reads `&` as `and`, takes no episode penalty for a show-level
  candidate, and scores an episode's series title equal but for a country qualifier
  (`The Traitors UK` against `The Traitors`) 0.9 with no year needed.
- `tvdb-series` declares its credential optional, so it is skipped without a PIN instead of listed
  as a missing key. This does not change matching: `tmdb-tv` comes first and already identified.
- The home layout's `recently-added` and catch-all grid titles name no content class: "Recently
  Added" and "All Titles", so a Films or TV library is not labelled "Movies & Episodes".

## Not in this change (needs SDK and Core)

- A matched episode titled with the series name and dated with its premiere year: identification
  writes the series lookup's `title` and `year` onto the episode, and Core counts a Media row as
  matched only when its title was written (`title_identified`). Dropping those mappings here would
  report every matched episode as unmatched. Core has to keep the parser's display title for a
  series-level match (the `core-1713-identify-keeps-parser-title` companion named by CORE-1713), and
  a per-episode lookup has to supply the episode's own air date.

- A per-episode lookup (`/tv/{id}/season/{s}/episode/{e}`): the scrape engine substitutes no season
  or episode placeholder, so it cannot be declared.
- Show and season hydration from the series lookup: an `EntityMap` needs a non-empty `Path`
  resolving to an element, so the lookup's root document cannot hydrate one show Entity, and the
  show Kind has no Media-targeted relation.
- The order of the `recently-added` Source and the title of a detail page's related row are Core
  behaviour: `SourceSpec` and `SectionSpec` declare no sort or related-row title.

- Folder corroboration of a non-exact series title: the scorer sees no path, so softening the
  no-year penalty when the show folder names the series needs a `ParsedMedia` field in the SDK.
- Region choice between same-title national versions: `Candidate` carries no origin country, and
  Core breaks an exact score tie by popularity, so `The Office (UK)` against two `The Office`
  candidates is decided by popularity in Core rather than failing closed.
- The library kind: `ParseContext` carries none, so a TV library is inferred from folder names.

## Impact

Core picks this up when it pins the plugin-filmtv release carrying it. No contract field changes.
