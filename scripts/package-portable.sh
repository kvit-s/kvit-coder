#!/bin/bash
set -euo pipefail

# Builds portable Windows zips: both .exes plus the local install.ps1 that
# adds the folder it runs from to the user's PATH.
#
#   scripts/package-portable.sh [version]
#
# With no version, git describe names it (or "dev" when tags are missing, as
# on a shallow CI checkout). The release job passes the tag, so the files it
# uploads carry the release version:
#
#   dist/portable/kvit-coder_<number>_windows_<arch>_portable.zip
#
# Each zip holds kvit-coder.exe, kvit-coder-ui.exe, install.ps1 (a copy of
# scripts/portable-install.ps1), config.example.yaml, README.md and LICENSE.
# Unzip anywhere and run its install.ps1 there; the folder stays the install
# location. goreleaser's own windows zips are left alone.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

usage() {
    cat <<'USAGE'
usage: scripts/package-portable.sh [version]

  version   release tag for file names and the version stamp
            (default: git describe --tags --always, or dev)
USAGE
}

case "${1:-}" in
    -h|--help) usage; exit 0 ;;
esac

version="${1:-$(git describe --tags --always 2>/dev/null || echo dev)}"
number="${version#v}"  # file names follow goreleaser: 0.3.0, not v0.3.0
commit_hash=$(git rev-parse --short HEAD 2>/dev/null || echo dev)
commit_date=$(git log -1 --format=%cd --date=format:%Y%m%d 2>/dev/null || echo unknown)
build_date=$(date -u +%Y-%m-%d)

if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    version="${version}-dirty"
fi

command -v go >/dev/null 2>&1 || { echo "package-portable.sh: needs go on the PATH" >&2; exit 1; }
command -v zip >/dev/null 2>&1 || { echo "package-portable.sh: needs zip on the PATH" >&2; exit 1; }

installer="$ROOT/scripts/portable-install.ps1"
[ -f "$installer" ] || { echo "package-portable.sh: $installer is missing" >&2; exit 1; }

ldflags="-s -w -X main.version=${version} -X main.commitHash=${commit_hash}"
ldflags="${ldflags} -X main.commitDate=${commit_date} -X main.buildDate=${build_date}"

out="$ROOT/dist/portable"
mkdir -p "$out"
rm -f "$out"/kvit-coder_*_windows_*_portable.zip

for goarch in amd64 arm64; do
    stage=$(mktemp -d)
    trap 'rm -rf "$stage"' EXIT
    echo "building windows/$goarch version ${version} commit ${commit_hash}"
    CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags="$ldflags" \
        -o "$stage/kvit-coder.exe" ./cmd/kvit-coder
    CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags="$ldflags" \
        -o "$stage/kvit-coder-ui.exe" ./cmd/kvit-coder-ui
    cp "$installer" "$stage/install.ps1"
    cp "$ROOT/config.example.yaml" "$ROOT/README.md" "$ROOT/LICENSE" "$stage/"
    archive="$out/kvit-coder_${number}_windows_${goarch}_portable.zip"
    (cd "$stage" && zip -j -9 -q "$archive" kvit-coder.exe kvit-coder-ui.exe install.ps1 config.example.yaml README.md LICENSE)
    trap - EXIT
    rm -rf "$stage"
done

echo "portable zips:"
ls -lh "$out"
if command -v sha256sum >/dev/null 2>&1; then
    (cd "$out" && sha256sum ./*.zip)
fi
