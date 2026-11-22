## 1. Build

- [x] 1.1 Tolerant full NFO reader with encoding-declaration support
- [x] 1.2 `season.nfo` lookup and the sidecar size+mtime signature
- [x] 1.3 `LocalNFOHydration` neutral plan (fields, credits, genre, studio, film-series, containers)
- [x] 1.4 Bundle Media fields and the film-series membership relation
- [x] 1.5 `nfo` migration source emits watch state

## 2. Prove

- [x] 2.1 Golden fixtures: Kodi film, tvshow + season + episode, multi-episode, Latin-1, truncated
- [x] 2.2 Watch state from `playcount`; bundle validates
- [x] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 Tag the plugin, then repin Core
