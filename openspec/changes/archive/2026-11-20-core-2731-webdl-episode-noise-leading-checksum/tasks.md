## 1. Build

- [x] 1.1 A hyphen-split release tag (`WEB-DL`) is not a release group in scene noise
- [x] 1.2 Release noise before an episode marker after a bare leading year names no series
- [x] 1.3 Leading checksums are blanked: no release group, no title cut, still episode context

## 2. Prove

- [x] 2.1 Table cases for the ticket paths, the checksum decision and controls
- [x] 2.2 `openspec validate --all --strict` passes
- [x] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [x] 3.1 Archive this change after landing, before completing the ticket
