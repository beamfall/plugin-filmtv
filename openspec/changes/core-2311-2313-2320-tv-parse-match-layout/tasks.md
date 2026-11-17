## 1. Build

- [x] 1.1 Parse `NeYY` season/episode codes without an `s`, with episode context
- [x] 1.2 Parse a leading `N NN` code inside an agreeing season folder or `Specials`
- [x] 1.3 Parse a zero-padded absolute episode number with episode context; group it under its
  season folder or Season 1, code `E01`
- [x] 1.4 Scorer ignores apostrophes, reads `&` as `and`, folds a country qualifier (no unique
  pass when only one side carries it) and takes 0.01 off a show-level candidate
- [x] 1.5 `tvdb-series` credential is optional
- [x] 1.7 Checksum brackets are release noise; image files are refused
- [x] 1.6 Home `recently-added` and catch-all titles name no content class

## 2. Prove

- [x] 2.1 Parser corpus fixtures for `1e01-Yesterday's Jam`, `3 01 From Hell`,
  `Dealbreakers Talk Show 0001`, and the refusals that keep movies safe
- [x] 2.2 Scorer and match-corpus tests: tower show titles auto-apply, national versions fail closed
- [x] 2.5a Parser tests for each reviewer false positive (camera clips, numbered film titles,
  `DD5.1e01`, `Specials`)
- [x] 2.3 Bundle tests: TheTVDB is optional, home titles name no class
- [x] 2.5b Tests for each round-2 reviewer input (one-sided qualifier, `This Is Us`, CRC-only
  anime, home and drone clips, film folders, group brackets, disagreeing season folder)
- [x] 2.4 `openspec validate --all --strict` passes
- [ ] 2.5 `script/gate.sh` passes on the landing head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the tickets
- [ ] 3.2 Core repins the plugin-filmtv release
