# Full Kodi-family NFO metadata import

Ticket: CORE-0361 (`queue:core:main`, DISC-FILMTV-NFO-FIELDS-1).
Status: proposed.

## Why

A library curated in Kodi, Jellyfin or Emby carries its metadata in `.nfo` sidecars. Today the
plugin reads only the title, year and provider IDs from them as identification hints, so every
curated plot, sort title, credit and genre is lost and a TMDB scrape replaces the user's own data.

## What Changes

- A tolerant NFO reader reads the whole Kodi family: the per-file `<stem>.nfo` (a film root or one
  or more episode roots for a multi-episode file), the folder film NFO, `season.nfo` and
  `tvshow.nfo`. It keeps every element that parsed before a malformed or truncated point, and honours
  the XML encoding declaration (ISO-8859-1 / Windows-1252).
- `LocalNFOHydration(path)` turns those files into a neutral sidecar plan for Core's hydration seam:
  built-in title, sort title, summary and year; typed Media fields; credits, genre, studio and
  film-series relations; container fields for the show and season the local mapper groups the file
  into; and a size+mtime signature of every NFO that fed the plan, so Core re-reads a changed NFO.
- The bundle declares the Media fields the NFO carries that it lacked (`outline`, `user-rating`,
  `tags`, `runtime`, `trailer`, `date-added`, the signature field) and a `film-series` → Media
  membership relation for `<set>`.
- The `nfo` migration source emits per-profile watch state from `playcount`, `watched` and
  `lastplayed`, and declares the watch-state category.

## Impact

- Code: `internal/mediaimport` (reader), `nfotree.go`, a new `nfohydration.go`, `sources.go`,
  `bundle.go`, golden fixtures under `testdata/nfo/`.
- Core consumes `LocalNFOHydration` and `NFOSignature` from its Film & TV hydration mapper and writes
  the plan at its `local` source tier (Core change under the same ticket).
