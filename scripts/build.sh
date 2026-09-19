#!/usr/bin/env bash
# Cross-compile keramos for the supported (OS, arch) matrix and emit a
# checksum file for the release. Binaries land in ./dist/.
#
# Usage:
#   scripts/build.sh                   # version derived from `git describe`
#   scripts/build.sh v1.0.0            # explicit version
#   VERSION=v1.0.0 scripts/build.sh    # via env

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT_DIR"

VERSION="${1:-${VERSION:-}}"
if [ -z "$VERSION" ]; then
    if VERSION="$(git describe --tags --exact-match 2>/dev/null)"; then
        :
    elif COMMIT="$(git rev-parse --short HEAD 2>/dev/null)"; then
        VERSION="dev-${COMMIT}"
    else
        VERSION="dev"
    fi
fi

COMMIT="$(git rev-parse HEAD 2>/dev/null || echo unknown)"
BUILD_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

DIST_DIR="$ROOT_DIR/dist"
rm -rf "$DIST_DIR"
mkdir -p "$DIST_DIR"

# (os, arch) matrix — amd64 and arm64 only, across every OS that
# Go supports both on. Targets that fail to compile are reported and
# skipped without aborting the build.
TARGETS=(
    "linux/amd64"
    "linux/arm64"
    "darwin/amd64"
    "darwin/arm64"
    "windows/amd64"
    "windows/arm64"
    "freebsd/amd64"
    "freebsd/arm64"
    "openbsd/amd64"
    "openbsd/arm64"
    "netbsd/amd64"
)

FAILED=()

LDFLAGS="-s -w \
-X github.com/ebogdum/keramos/v3/internal/cli.Version=${VERSION} \
-X github.com/ebogdum/keramos/v3/internal/cli.Commit=${COMMIT} \
-X github.com/ebogdum/keramos/v3/internal/cli.BuildDate=${BUILD_DATE}"

echo "Building keramos ${VERSION} (commit ${COMMIT:0:12})"
echo

for target in "${TARGETS[@]}"; do
    GOOS="${target%/*}"
    GOARCH="${target#*/}"
    NAME="keramos-${VERSION}-${GOOS}-${GOARCH}"
    EXT=""
    if [ "$GOOS" = "windows" ]; then
        EXT=".exe"
    fi
    OUT_DIR="$DIST_DIR/$NAME"
    mkdir -p "$OUT_DIR"
    OUT_BIN="$OUT_DIR/keramos$EXT"

    printf "  %-22s " "$GOOS/$GOARCH"
    if ! GOOS="$GOOS" GOARCH="$GOARCH" CGO_ENABLED=0 \
        go build -trimpath -ldflags "$LDFLAGS" -o "$OUT_BIN" ./cmd/keramos \
        > "$DIST_DIR/$NAME.build.log" 2>&1; then
        printf "SKIP (compile failure — see %s.build.log)\n" "$NAME"
        FAILED+=("$GOOS/$GOARCH")
        rm -rf "$OUT_DIR"
        continue
    fi
    rm -f "$DIST_DIR/$NAME.build.log"
    cp "$ROOT_DIR/LICENSE"   "$OUT_DIR/"
    cp "$ROOT_DIR/README.md" "$OUT_DIR/"

    if [ "$GOOS" = "windows" ]; then
        ARCHIVE="$NAME.zip"
        (cd "$DIST_DIR" && zip -q -r "$ARCHIVE" "$NAME")
    else
        ARCHIVE="$NAME.tar.gz"
        (cd "$DIST_DIR" && tar -czf "$ARCHIVE" "$NAME")
    fi
    rm -rf "$OUT_DIR"

    SIZE="$(wc -c < "$DIST_DIR/$ARCHIVE" | tr -d ' ')"
    printf "ok  %s (%s bytes)\n" "$ARCHIVE" "$SIZE"
done

if [ "${#FAILED[@]}" -gt 0 ]; then
    echo
    echo "Skipped (compile failure):"
    for t in "${FAILED[@]}"; do echo "  - $t"; done
fi

echo
echo "Computing SHA256 checksums..."
(
    cd "$DIST_DIR"
    if command -v sha256sum >/dev/null 2>&1; then
        sha256sum keramos-${VERSION}-* > SHA256SUMS
    else
        shasum -a 256 keramos-${VERSION}-* > SHA256SUMS
    fi
)

echo
echo "Artifacts in $DIST_DIR:"
ls -la "$DIST_DIR"
