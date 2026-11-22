# Full Kodi-family NFO metadata import

Ticket: CORE-0361 (`queue:core:main`, DISC-FILMTV-NFO-FIELDS-1).
Status: proposed.

## Why

A library curated in Kodi, Jellyfin or Emby carries its metadata in `.nfo` sidecars. Today the
plugin reads only the title, year and provider IDs from them as identification hints, so every
curated plot, sort title, credit and genre is lost and a TMDB scrape replaces the user's own data.

## What Changes

- A tolerant NFO reader reads the per-file `<stem>.nfo` (a film root or one or more episode roots
  for a multi-episode file) and the folder film NFO. It keeps every element that parsed before a
  malformed or truncated point, honours the XML encoding declaration (ISO-8859-1 / Windows-1252),
  and accepts `<collection>` as well as `<set>`.
- `LocalNFOHydration(path)` turns that file into a neutral sidecar plan for Core's hydration seam:
  built-in title, sort title, summary and year; typed Media fields; credits, genre, studio and
  film-series relations; the signature of the exact bytes it parsed; and a distinct unreadable
  state, so Core never treats a failed read as a removed sidecar.
- A folder film NFO describes the main feature only: trailers and other extras in the folder take
  neither its metadata nor its watch state.
- Only a US-qualified or unqualified rating becomes the policy certification; a foreign-only rating
  is display text.
- The bundle declares the Media fields the NFO carries that it lacked (`outline`, `user-rating`,
  `tags`, `runtime`, `trailer`, `date-added`, the signature field) and a `film-series` → Media
  membership relation for `<set>`.
- The `nfo` migration source emits per-profile watch state from `playcount`, `watched` and
  `lastplayed`, only when the run selected the watch-state category.

## Cut from this change

Container NFO import (`tvshow.nfo` and `season.nfo` fields and relations) is cut. Entity fields have
no per-field provenance in Core, so a container value read from a sidecar cannot outrank a later
scrape or yield to a manager's edit. It returns under its own Core ticket once Entity fields carry
provenance. Episode and film Media fields stay in scope.

The play count itself is not carried: the SDK `WatchState` has no play-count field, and adding one is
an SDK change outside this plugin.

## Impact

- Code: `internal/mediaimport` (reader), `nfotree.go`, a new `nfohydration.go`, `sources.go`,
  `bundle.go`, golden fixtures under `testdata/nfo/`.
- Core consumes `LocalNFOHydration`, `NFOSignature` and `NFOOwnedRelations` from its Film & TV
  hydration mapper and writes the plan at its `local` source tier (Core change under the same
  ticket).
