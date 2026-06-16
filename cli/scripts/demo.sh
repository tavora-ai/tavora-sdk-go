#!/usr/bin/env bash
# Convex-style deployment-versioning demo, driven entirely through the
# `tavora` CLI. Walks: init → dev (sync a draft) → deploy (cut a
# version, dev runs it) → promote to staging → promote to prod →
# status, then a rollback with --version.
#
# Prerequisites: a reachable tavora backend and credentials.
#   export TAVORA_URL=http://localhost:8091
#   tavora login                # or: export TAVORA_API_KEY=tk_...
#
# Run from the repo root: `mise run demo` (or `bash cli/scripts/demo.sh`).
set -euo pipefail

PROJECT="${DEMO_PROJECT:-demoflow}"
WORKDIR="$(mktemp -d -t tavora-demo-XXXXXX)"
trap 'rm -rf "$WORKDIR"' EXIT

# Always build the CLI from this repo's source — the demo showcases
# THIS checkout's behavior, not whatever `tavora` happens to be on PATH.
# Build outside WORKDIR so the binary doesn't collide with the `tavora/`
# project folder `tavora init` scaffolds inside WORKDIR.
BINDIR="$(mktemp -d -t tavora-bin-XXXXXX)"
trap 'rm -rf "$WORKDIR" "$BINDIR"' EXIT
echo "→ building tavora from source"
( cd "$(dirname "$0")/.." && go build -o "$BINDIR/tavora" ./cmd/tavora )
TAVORA="$BINDIR/tavora"

if [ -z "${TAVORA_API_KEY:-}" ] && [ ! -f "$HOME/.tavora.yaml" ]; then
  echo "No credentials found. Set TAVORA_API_KEY or run \`tavora login\` first." >&2
  echo "TAVORA_URL is currently: ${TAVORA_URL:-<unset, defaults to http://localhost:8080>}" >&2
  exit 1
fi

step() { printf '\n\033[1;36m==> %s\033[0m\n' "$*"; }

cd "$WORKDIR"

step "tavora init — scaffold a project"
"$TAVORA" init --project "$PROJECT" >/dev/null
cd "$WORKDIR/tavora"

step "tavora dev --once — sync a dev draft"
"$TAVORA" dev --once --no-init

step "tavora deploy — cut v1; your dev environment now runs it"
"$TAVORA" deploy

step "tavora status — dev should show v1; staging/prod still empty"
"$TAVORA" status

step "tavora promote --to staging — ship the latest version to staging"
"$TAVORA" promote --to staging

step "tavora promote --to prod — ship it to production"
"$TAVORA" promote --to prod

step "tavora status — every environment now serves v1"
"$TAVORA" status

step "edit + re-deploy — cut v2 (dev follows; staging/prod stay pinned)"
echo "// demo edit $(date +%s)" >> agents/starter/persona.md
"$TAVORA" dev --once --no-init >/dev/null
"$TAVORA" deploy
"$TAVORA" status

step "tavora promote --to prod --version 1 — roll production back to v1"
"$TAVORA" promote --to prod --version 1
"$TAVORA" status

printf '\n\033[1;32mDone.\033[0m dev=latest, prod pinned to v1 — the Convex promote/rollback model, end to end.\n'
