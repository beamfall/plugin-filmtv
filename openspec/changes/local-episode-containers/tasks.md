## 1. Bundle

- [ ] 1.1 Declare the `season-episode` and `season-of` relations on the season Kind
- [ ] 1.2 Declare the `filmtv-hydrated` flag Media field
- [ ] 1.3 Add a "Shows" grid over show Entities to the library home

## 2. Parser

- [ ] 2.1 Add `LocalHydration(path)`
- [ ] 2.2 Implement `MediaTitle(path, probe)`

## 3. Tests

- [ ] 3.1 Unit-test `LocalHydration` and `MediaTitle` over the parser corpus shapes
- [ ] 3.2 Pin the new relations, field and section in the bundle tests

## 4. Verify and close

- [ ] 4.1 `openspec validate --all --strict`
- [ ] 4.2 `script/gate.sh`
