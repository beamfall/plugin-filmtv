## 1. Build

- [ ] 1.1 Rename the `show` kind's `overview` to `summary` and declare the new detail fields
- [ ] 1.2 Extend the `tmdb-tv` show container map: fields, backdrop, title path and container relations

## 2. Prove

- [ ] 2.1 Bundle tests pin the show container map, and the bundle validates against the SDK change
- [ ] 2.2 A scraper reference-trace vector freezes the show container output (accented title)
- [ ] 2.3 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 After plugin-sdk is tagged, raise `go.mod`, `SDKFloor` and `SDKCeiling` to it, tag, then repin core
- [ ] 3.2 Rescan on the Tower: the 3 Body Problem show page shows a backdrop, genres, Netflix, 2024 and cast
