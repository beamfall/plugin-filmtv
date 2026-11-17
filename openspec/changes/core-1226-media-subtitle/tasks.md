## 1. Build

- [x] 1.1 Declare `MediaSubtitle` on the bundle
- [x] 1.2 Map the episode `name` onto `media.episode-title` in the `tmdb-tv` item lookup

## 2. Prove

- [x] 2.1 Bundle tests pin the template, the item field map and a rendered episode subtitle
- [x] 2.2 `openspec validate --all --strict` passes
- [x] 2.3 `script/gate.sh` passes

## 3. Release (owner approval)

- [ ] 3.1 After plugin-sdk is tagged, raise `go.mod`, `SDKFloor` and `SDKCeiling` to it, tag, then repin core
- [ ] 3.2 Archive this change after landing, before completing the ticket
