## 1. Build

- [ ] 1.1 A hyphen-split release tag (`WEB-DL`) is not a release group in scene noise
- [ ] 1.2 Release noise before an episode marker after a bare leading year names no series
- [ ] 1.3 Leading checksums are blanked: no release group, no title cut, still episode context

## 2. Prove

- [ ] 2.1 Table cases for the ticket paths, the checksum decision and controls
- [ ] 2.2 `openspec validate --all --strict` passes
- [ ] 2.3 `script/gate.sh` passes on the lane head

## 3. Close

- [ ] 3.1 Archive this change after landing, before completing the ticket
