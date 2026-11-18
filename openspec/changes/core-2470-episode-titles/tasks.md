## 1. Build

- [x] 1.1 Add the dynamic-range, audio and streaming-source tags to the release vocabulary
- [x] 1.2 Implement `IdentifiedMediaTitle`: the identified `episode-title` replaces the filename's

## 2. Prove

- [x] 2.1 Parser tests with the Ted Lasso names: `dv`/`hdr` never the title, real titles kept
- [x] 2.2 `IdentifiedMediaTitle` test: identified name with punctuation, bare code named, fallback
- [x] 2.3 `openspec validate --all --strict` passes
- [ ] 2.4 Scoped checks (go test, gofmt, golangci-lint) pass on the lane head

## 3. Close

- [ ] 3.1 Release, then core repins it with the core identified-title change
- [ ] 3.2 Archive this change after landing, before completing the ticket
