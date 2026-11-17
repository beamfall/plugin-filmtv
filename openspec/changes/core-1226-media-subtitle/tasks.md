## 1. Build

- [ ] 1.1 Declare `MediaSubtitle` on the bundle
- [ ] 1.2 Map the episode `name` onto `media.episode-title` in the `tmdb-tv` item lookup

## 2. Prove

- [ ] 2.1 Bundle tests pin the template, the item field map and a rendered episode subtitle
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes

## 3. Release (owner approval)

- [ ] 3.1 After plugin-sdk is tagged, raise `go.mod`, `SDKFloor` and `SDKCeiling` to it, tag, then repin core
- [ ] 3.2 Archive this change after landing, before completing the ticket
