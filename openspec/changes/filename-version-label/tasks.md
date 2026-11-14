## 1. Build

- [x] 1.1 Fill `ParsedMedia.VersionLabel` in the filename parser
- [ ] 1.2 Pin the plugin-sdk release that carries `VersionLabel` in `go.mod`

## 2. Prove

- [x] 2.1 Version label table test, grouping-key test, conformance vector and fuzz contract check
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
