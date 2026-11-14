## 1. Bundle

- [x] 1.1 Declare the `tvmaze-show` scraper with its attribution and request spacing
- [x] 1.2 Declare the `anilist-series` and `anilist-movie` scrapers behind an optional token

## 2. Absolute order

- [x] 2.1 Map an absolute episode number to a season and episode over regular episodes
- [x] 2.2 Decode TVmaze's embedded episode list

## 3. Tests

- [x] 3.1 Bundle, attribution, credential and spacing tests
- [x] 3.2 Scraper trace vectors over recorded documents
- [x] 3.3 Mapper vectors over a recorded TVmaze episode list

## 4. Verify and close

- [x] 4.1 `openspec validate --all --strict`
- [ ] 4.2 `script/gate.sh`
- [ ] 4.3 `openspec archive anime-metadata-providers` after landing
