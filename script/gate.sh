#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."

run_contracts() {
  echo "== test-depth contracts =="
  python3 .github/workflows/test_depth_gate.py contracts
}

run_coverage() {
  echo "== one-pass per-package coverage =="
  python3 .github/workflows/test_depth_gate.py coverage
}

run_govulncheck() {
  echo "== govulncheck (pinned tools module) =="
  if ! go -C tools tool 2>/dev/null | grep -q '(golang.org/x/vuln/cmd/govulncheck)$'; then
    echo "govulncheck: tools module does not declare golang.org/x/vuln/cmd/govulncheck" >&2
    exit 1
  fi
  go -C tools tool govulncheck -C .. ./...
}

run_fast_gate() {
  echo "== gofmt =="
  fmt_out="$(gofmt -l .)"
  if [ -n "$fmt_out" ]; then
    echo "gofmt: files need formatting:"
    echo "$fmt_out"
    exit 1
  fi

  echo "== go vet =="
  go vet ./...

  echo "== test-depth contracts + one-pass per-package coverage =="
  python3 .github/workflows/test_depth_gate.py check

  echo "== golangci-lint =="
  golangci-lint run ./...

  run_govulncheck

  echo "ALL GATES GREEN"
}

run_fuzz() {
  fuzz_package="${FUZZ_PACKAGE:-}"
  fuzz_target="${FUZZ_TARGET:-}"
  fuzz_time="${FUZZ_TIME:-20s}"

  if [[ ! "$fuzz_time" =~ ^([1-9][0-9]*)(s|x)$ ]]; then
    echo "fuzz: FUZZ_TIME must be a positive whole-second or execution budget, got $fuzz_time" >&2
    exit 1
  fi
  python3 .github/workflows/test_depth_gate.py fuzz-target "$fuzz_package" "$fuzz_target"
  go test -run '^$' -fuzz "^${fuzz_target}$" -fuzztime="$fuzz_time" "$fuzz_package"
}

case "${1:-gate}" in
  gate)
    run_fast_gate
    ;;
  govulncheck)
    run_govulncheck
    ;;
  contracts)
    run_contracts
    ;;
  coverage)
    run_coverage
    ;;
  race)
    echo "== race =="
    go test -race -count=1 ./...
    ;;
  fuzz)
    echo "== bounded parser fuzz =="
    run_fuzz
    ;;
  *)
    echo "usage: $0 [gate|govulncheck|contracts|coverage|race|fuzz]" >&2
    exit 2
    ;;
esac
