# Episode titles: release tags are never the title; identification names the episode

Ticket: CORE-2470 (`queue:core:main`). Related: CORE-2026 (existing media keeps old titles).
Status: proposed.

## Why

Tower QA (core 3efdff24, filmtv v0.1.17) showed three wrong episode titles on a real Ted Lasso
library:

- `ted.lasso.s03e08.dv.2160p.web.h265-ggez.mkv` is titled `S03E08 · dv`, and S03E09 and S03E10
  `· hdr`: `DV`, `HDR` and their kin are not in the release vocabulary, so the parser takes them
  as the episode's name. This breaks the existing "Episode title field" requirement (the text
  before the first release tag).
- `Ted.Lasso.S03E02.I.Dont.Want.to.Go.to.Chelsea...` keeps the filename's `I Dont Want to Go to
  Chelsea`: a dotted name cannot carry the apostrophe the identified episode name has.
- `S03E01` stays a bare code although identification wrote `Smells Like Mean Spirit` to
  `episode-title`: the display title is read from the filename only.

## What Changes

- The release vocabulary gains dynamic-range, audio and streaming-source tags: `dovi`, `hdr10+`,
  `hdr10plus`, `hlg`, `sdr`, `ddp`, `avc`, `atvp`, `amzn`, `dsnp`, `hmax` always end a title;
  `dv`, `hdr` and `atmos` end it before more release tags or as the last word, so a real title
  that contains one ("The HDR Story") keeps it.
- The parser implements Core's optional identified-title hook,
  `IdentifiedMediaTitle(path, probe, fields)`: when an identification's plugin field values carry
  `episode-title` (the `tmdb-tv` item lookup's episode `name`), the display title is the episode
  code, ` · ` and that name; otherwise it is `MediaTitle`.

## Impact

- Code: `parse.go` (vocabulary), `local.go` (`IdentifiedMediaTitle`).
- Core uses the hook once it repins this release and its identification apply path passes the
  identified field values (CORE-2470 core change). Media identified before then keeps its title
  until it is identified again (refresh or Fix match); a backfill is CORE-2026's.
