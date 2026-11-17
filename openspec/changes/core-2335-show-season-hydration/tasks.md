## 1. Build

- [ ] 1.1 Declare `overview` on the `show` and `season` kinds
- [ ] 1.2 Declare the show and season container maps on `tmdb-tv`
- [ ] 1.3 Declare the per-episode item lookup on `tmdb-tv`

## 2. Prove

- [ ] 2.1 Bundle tests pin the item lookup and both container maps, and the bundle validates
- [ ] 2.2 `openspec validate --all --strict` passes

## 3. Release (owner approval)

- [ ] 3.1 After plugin-sdk is tagged, raise `go.mod`, `SDKFloor` and `SDKCeiling` to it, tag, then repin core
