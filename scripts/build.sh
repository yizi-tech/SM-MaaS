#!/usr/bin/env sh
# Production build for MASS. Output binaries land in dist/bin/.
# Usage: scripts/build.sh
set -eu

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
mkdir -p "$ROOT/dist/bin"

cd "$ROOT/backend"
echo "==> building mass-server ..."
CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o "$ROOT/dist/bin/mass-server" ./cmd/server
echo "==> building seed-demo-users ..."
CGO_ENABLED=0 go build -trimpath -ldflags="-w -s" -o "$ROOT/dist/bin/seed-demo-users" ./cmd/seed-demo-users

echo "==> done:"
ls -lh "$ROOT/dist/bin"