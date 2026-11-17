## 1. Build

- [x] 1.1 Parse `NeYY` season/episode codes without an `s`
- [x] 1.2 Parse a leading `N NN` code inside an agreeing season folder
- [x] 1.3 Parse a trailing zero-padded absolute episode number; group it under Season 1, code `E01`
- [x] 1.4 Scorer ignores apostrophes when comparing titles
- [x] 1.5 `tvdb-series` credential is optional
- [x] 1.6 Home `recently-added` and catch-all titles name no content class

## 2. Prove

- [x] 2.1 Parser corpus fixtures for `1e01-Yesterday's Jam`, `3 01 From Hell`,
  `Dealbreakers Talk Show 0001`, and the refusals that keep movies safe
- [x] 2.2 Scorer test: apostrophe-free scene title scores exact against the provider title
- [x] 2.3 Bundle tests: TheTVDB is optional, home titles name no class
- [x] 2.4 `openspec validate --all --strict` passes
- [ ] 2.5 `script/gate.sh` passes on the landing head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the tickets
- [ ] 3.2 Core repins the plugin-filmtv release
