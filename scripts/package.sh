#!/usr/bin/env sh
# Package the production-grade MASS source release. Produces:
#   dist/mass-platform-<version>-src.tar.gz
#   dist/mass-platform-<version>-src.zip
#   dist/SHA256SUMS.txt
# Usage: scripts/package.sh [version]
set -eu

VERSION="${1:-1.0.0}"
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
DIST="$ROOT/dist"
mkdir -p "$DIST"

NAME="mass-platform-$VERSION-src"
TGZ="$DIST/$NAME.tar.gz"
ZIP="$DIST/$NAME.zip"

# Global exclude patterns: dev binaries/logs, runtime data, scratch files.
EXCLUDES="
--exclude=*.exe
--exclude=*.log
--exclude=./uploads
--exclude=./backend/uploads
--exclude=./dist
--exclude=home_redesign.html
--exclude=落地页示例.html
--exclude=./debug-auth-login-refused.md
--exclude=./docs/debug-auth-login-refused.md
"

cd "$ROOT"
echo "==> creating $TGZ"
tar -a -cf "$TGZ" $EXCLUDES -C "$ROOT" .
echo "==> creating $ZIP"
tar -a -cf "$ZIP" $EXCLUDES -C "$ROOT" .

echo "==> writing checksums"
(cd "$DIST" && sha256sum "$NAME.tar.gz" "$NAME.zip" > SHA256SUMS.txt)

echo "==> done:"
(cd "$DIST" && ls -lh "$NAME.tar.gz" "$NAME.zip" SHA256SUMS.txt)