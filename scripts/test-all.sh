#!/usr/bin/env bash
#
# scripts/test-all.sh — OFFLINE automated test suite for the buktio monorepo.
#
# Runs every check that does NOT need Docker, a database, or a running server:
#   - gofmt -l           (fail if any file is unformatted)
#   - go build ./...     (main module compiles)
#   - go vet ./...       (main module static checks)
#   - go test ./...      (unit tests; -race -short, mocked Garage)
#   - go build ./...     for operators/buktio-operator      (separate module)
#   - go build ./...     for terraform-provider-buktio       (separate module)
#   - (cd apps/web && pnpm typecheck)                        (if pnpm is present)
#
# It does NOT boot servers, does NOT touch Docker, and does NOT touch any
# database. For the manual end-to-end flows (per edition) see docs/TESTING.md.
#
# Failures are collected and reported at the end; the script exits non-zero
# if ANY check failed.

set -euo pipefail

# --- Go toolchain (not installed system-wide; lives under /tmp) -------------
export GOROOT=/tmp/goroot/go
export GOPATH=/tmp/gopath
export PATH=/tmp/goroot/go/bin:$PATH
export GOTOOLCHAIN=local

# --- Locate the repo root (this script lives in <root>/scripts) -------------
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
cd "$ROOT"

# --- Result collection ------------------------------------------------------
FAILED=()
PASSED=()

section() {
  echo
  echo "============================================================"
  echo "==> $1"
  echo "============================================================"
}

# run <label> <command...> — run a check, record pass/fail, keep going.
run() {
  local label="$1"; shift
  section "$label"
  if "$@"; then
    echo "--- PASS: $label"
    PASSED+=("$label")
  else
    echo "--- FAIL: $label"
    FAILED+=("$label")
  fi
}

# --- gofmt: must report zero unformatted files ------------------------------
check_gofmt() {
  local out
  out="$(gofmt -l .)"
  if [ -n "$out" ]; then
    echo "gofmt needed on:"
    echo "$out"
    return 1
  fi
  echo "all files formatted"
  return 0
}

# --- separate-module build: build ./... inside a sibling Go module ----------
build_module() {
  local dir="$1"
  ( cd "$ROOT/$dir" && go build ./... )
}

# --- web typecheck (only if pnpm exists) ------------------------------------
web_typecheck() {
  if ! command -v pnpm >/dev/null 2>&1; then
    echo "pnpm not found on PATH — SKIPPING web typecheck"
    return 0
  fi
  ( cd "$ROOT/apps/web" && pnpm typecheck )
}

# --- Toolchain banner -------------------------------------------------------
section "Go toolchain"
go version || { echo "go toolchain not found at $GOROOT"; exit 1; }

# --- Run the offline suite (continue-and-collect) ---------------------------
run "gofmt -l (main module)"                 check_gofmt
run "go build ./... (main module)"           go build ./...
run "go vet ./... (main module)"             go vet ./...
run "go test ./... -race -short (main)"      go test ./... -race -short
run "go build ./... (buktio-operator)"       build_module "operators/buktio-operator"
run "go build ./... (terraform-provider)"    build_module "terraform-provider-buktio"
run "pnpm typecheck (apps/web)"              web_typecheck

# --- Summary ----------------------------------------------------------------
section "SUMMARY"
echo "PASSED (${#PASSED[@]}):"
for p in "${PASSED[@]:-}"; do [ -n "$p" ] && echo "  ok    $p"; done

if [ "${#FAILED[@]}" -gt 0 ]; then
  echo
  echo "FAILED (${#FAILED[@]}):"
  for f in "${FAILED[@]}"; do echo "  FAIL  $f"; done
  echo
  echo "RESULT: FAIL"
  echo
  echo "Next: manual E2E — see docs/TESTING.md"
  exit 1
fi

echo
echo "RESULT: PASS — all offline checks passed."
echo
echo "Next: manual E2E — see docs/TESTING.md"
echo "  (per-edition flows: OSS / Pro / Enterprise / Hosted; need the dev infra up)"
