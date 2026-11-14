# Read a version label from `Title (Year) - <label>` filenames

Ticket: CORE-1676 (`queue:core:main`).
Status: proposed.

## Why

Operators keep several versions of one movie side by side, named the common way:
`Title (Year) - 1080p.mkv`, `Title (Year) - 2160p.mkv`, `Title (Year) - Director's Cut.mkv`. Core
groups such files under one Media as Versions, but only when the plugin says which version each file
is. The plugin SDK now carries that as `ParsedMedia.VersionLabel` (plugin-sdk `parsed-version-label`).

## What Changes

- The filename parser fills `VersionLabel` for a non-episode name whose parenthesized year is followed
  by a spaced hyphen and a label. One layer of enclosing brackets is dropped and the label is bounded
  to the SDK maximum.
- No label for: dotted release names, names without a parenthesized year, episodes, stacked parts
  (`cd1`, `part 2`, `disc1`) and extras (`trailer`, `sample`, `featurette`, ...).
- Title, class and year are unchanged, so every version of a title parses to the same grouping key.

## Impact

Needs plugin-sdk with `ParsedMedia.VersionLabel` (v0.3.5 or later). No existing corpus case changes.
