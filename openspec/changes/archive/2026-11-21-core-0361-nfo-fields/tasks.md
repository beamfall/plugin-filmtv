## 1. Build

- [x] 1.1 Tolerant full NFO reader with encoding-declaration support, `<collection>` and pre-1900 years
- [x] 1.2 Signature of the parsed bytes, the unreadable state and the cheap size+mtime signature
- [x] 1.3 `LocalNFOHydration` neutral plan (fields, credits, genre, studio, film-series) and the owned relations
- [x] 1.4 Bundle Media fields and the film-series membership relation
- [x] 1.5 `nfo` migration source emits watch state for the main feature, only when selected
- [x] 1.6 US-first certification selection

## 2. Prove

- [x] 2.1 Golden fixtures: Kodi film, folder film, episode, multi-episode, Latin-1, truncated
- [x] 2.2 Watch state from `playcount`, category selection, extras; bundle validates
- [x] 2.3 Unreadable and changed-while-read sidecars; certification selection
- [x] 2.4 `openspec validate --all --strict` passes
- [x] 2.5 Lookup errors and oversized files read as unreadable; watch-state source fails on them
- [x] 2.6 Space-separated sample, split director and credits, non-finite ratings, tags from every root, country-name qualifiers
- [x] 2.7 Absolute-path signature; resume skips NFO reads before the checkpoint

## 3. Release (owner approval)

- [ ] 3.1 Tag the plugin, then repin Core
