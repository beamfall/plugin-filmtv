## 1. Bundle

- [x] 1.1 Declare the `season-episode` and `season-of` relations on the season Kind
- [x] 1.2 Declare the `filmtv-hydrated` flag Media field
- [x] 1.3 Add a "Shows" grid over show Entities to the library home

## 2. Parser

- [x] 2.1 Add `LocalHydration(path)`
- [x] 2.2 Implement `MediaTitle(path, probe)`

## 3. Tests

- [x] 3.1 Unit-test `LocalHydration` and `MediaTitle` over the parser corpus shapes
- [x] 3.2 Pin the new relations, field and section in the bundle tests

## 4. Verify and close

- [x] 4.1 `openspec validate --all --strict`
- [x] 4.2 `script/gate.sh`
