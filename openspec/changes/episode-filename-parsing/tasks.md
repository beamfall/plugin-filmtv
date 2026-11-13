## 1. Parser

- [x] 1.1 Trim separators and a leading release-group bracket from the series title
- [x] 1.2 Extract the episode title and emit it through `ParseMediaFields`
- [x] 1.3 Report the last episode of a multi-episode chain as the index end
- [x] 1.4 Take the series name from the folder above a season folder when the file has none
- [x] 1.5 Widen release-tag detection; treat word-like tags as tags only before another tag

## 2. Bundle

- [x] 2.1 Declare the `episode-title` Media field

## 3. Tests

- [x] 3.1 Extend the parser corpus and add episode-title vectors
- [x] 3.2 Add episode-title invariants and seeds to `FuzzParseFilename`

## 4. Verify and close

- [x] 4.1 `openspec validate --all --strict`
- [x] 4.2 `script/gate.sh` and `script/gate.sh fuzz` for `FuzzParseFilename`
- [ ] 4.3 `openspec archive episode-filename-parsing` after landing
