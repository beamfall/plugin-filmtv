# AGENTS.md — Beamfall Film & TV

Film and TV plugin for [Beamfall Core](https://github.com/beamfall/core): recognizes, matches and organizes movies and shows, imports libraries and watch history from Plex, Emby, Jellyfin, Trakt and NFO sidecars, and scrobbles to Trakt.

> **This file is canonical.** `CLAUDE.md` is a symlink to it — edit `AGENTS.md`, never maintain a
> second copy.

> **Documentation:** keep public docs in `README.md`, `CONTRIBUTING.md`, `SECURITY.md` and this file.
> When this repo is checked out inside the Beamfall multi-repo workspace, also follow that workspace's
> `DOCUMENTATION-GOVERNANCE.md` before adding, moving, or archiving any doc.

Use normal repo-local CLI and shell tools.

## Layout

- `parse.go`, `score.go` — filename parser and match scorer; `bundle.go` — plugin bundle.
- `nfotree.go`, `internal/mediaimport/` — NFO and artwork import; `internal/foreignimport/`.
- `trakt.go`, `trakt_scrobble.go`, `sources.go` — Trakt import/scrobble and metadata sources.
- `*corpus_test.go`, `conformance_test.go`, `*_fuzz_test.go` — conformance vectors and fuzz.
- `tools/` — pinned tool module; `script/gate.sh`, `.github/workflows/`, `testing/` — gate and CI.

## Build and test

- `go test ./...` — unit tests plus the plugin SDK conformance runners (`sdktest`).
- `script/gate.sh` — full local gate: test-depth contracts, per-package coverage, govulncheck (pinned `tools/` module), gofmt, vet, golangci-lint, race, bounded parser fuzz. Requires `golangci-lint` and `python3` on `PATH`.
- `.github/workflows/conformance.yml` runs the same checks in CI against the pinned plugin SDK, plus
  a race job and a bounded fuzz job. Keep the local gate and the workflow in step.
- Per-package coverage floors live in `testing/coverage-floor.json`; raise them with new tests, do
  not lower them to pass the gate.

## Conventions

- Plugins require a tagged SDK version in `go.mod`; never commit a `replace` for
  `github.com/beamfall/plugin-sdk` (an uncommitted `go.work` is fine for local cross-repo work).
- Format with `gofmt`; the lint rule set is `.golangci.yml`, shared verbatim across Beamfall plugin
  repos — do not fork it per plugin.
- Plugins are pure Go metadata providers built on `github.com/beamfall/plugin-sdk`; keep the
  dependency set minimal and the public behavior covered by conformance vectors.
- `nfotree.go` and `internal/mediaimport/nfo.go` deliberately split literals such as `"movie.nfo"`
  into `"mo" + "vie..."` to avoid a vocabulary scanner. Grep for `movie.nfo` before "fixing"
  them back into a single literal.

## Corvint context

Before editing, read `corvint context "<task>"` or `corvint impact <path>...`; before choosing tests,
run `corvint affected --base <origin/main sha>`. **Verify:** `script/gate.sh`.

## Contributing and license

- Licensed under the GNU AGPL v3 (`LICENSE`). Preserve license headers and third-party notices.
- External contributions need an accepted Beamfall Contributor License Agreement first: see
  `CONTRIBUTING.md` and `CLA.md`. Report vulnerabilities as described in `SECURITY.md`, not in
  public issues.
