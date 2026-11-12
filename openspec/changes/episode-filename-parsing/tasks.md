## 1. Parser

- [ ] 1.1 Trim separators and a leading release-group bracket from the series title
- [ ] 1.2 Extract the episode title and emit it through `ParseMediaFields`
- [ ] 1.3 Report the last episode of a multi-episode chain as the index end
- [ ] 1.4 Take the series name from the folder above a season folder when the file has none
- [ ] 1.5 Widen release-tag detection; treat word-like tags as tags only before another tag

## 2. Bundle

- [ ] 2.1 Declare the `episode-title` Media field

## 3. Tests

- [ ] 3.1 Extend the parser corpus and add episode-title vectors
- [ ] 3.2 Add episode-title invariants and seeds to `FuzzParseFilename`

## 4. Verify and close

- [ ] 4.1 `openspec validate --all --strict`
- [ ] 4.2 `script/gate.sh` and `script/gate.sh fuzz` for `FuzzParseFilename`
- [ ] 4.3 `openspec archive episode-filename-parsing` after landing
