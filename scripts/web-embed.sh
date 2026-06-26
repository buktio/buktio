#!/usr/bin/env bash
# web-embed.sh — build the web panel as a static export and embed it into
# internal/webui/dist, so the Go binaries (buktio-api, buktio-api-ee) ship the UI
# inside a single binary with no separate Node web container.
#
# Invariant I3 (roadmap): any UI change must re-embed before the binary is built.
# CI runs this before building the api images; `make build-api`/`build-api-ee`
# depend on it (skip with EMBED=0 for fast Go-only iteration).
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
WEB="$ROOT/apps/web"
DIST="$ROOT/internal/webui/dist"
PNPM="${PNPM:-pnpm}"

echo ">> web-embed: building static export (apps/web)"
(
  cd "$WEB"
  if [ ! -d node_modules ]; then
    "$PNPM" install --frozen-lockfile
  fi
  "$PNPM" build
)

if [ ! -f "$WEB/out/index.html" ]; then
  echo "!! web-embed: apps/web/out/index.html missing — static export failed" >&2
  exit 1
fi

echo ">> web-embed: copying export into internal/webui/dist"
# Wipe the previously embedded output, preserving the tracked placeholder + .gitignore.
find "$DIST" -mindepth 1 -maxdepth 1 ! -name placeholder.html ! -name .gitignore -exec rm -rf {} +
cp -R "$WEB/out/." "$DIST/"

echo ">> web-embed: embedded $(find "$DIST" -type f | wc -l | tr -d ' ') files"
