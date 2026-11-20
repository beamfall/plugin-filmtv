## 1. Build

- [x] 1.1 Checksum-only show folders give no series name
- [x] 1.2 Scene tag before checksums and release tags is noise; no title after a bare leading year
- [x] 1.3 Unicode word boundaries for years in findYear and endsInYearWord

## 2. Prove

- [x] 2.1 Table cases for the ticket paths and controls
- [x] 2.2 `openspec validate --all --strict` passes
- [x] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [x] 3.1 Archive this change after landing, before completing the ticket
