#!/bin/bash
set -euo pipefail

# Builds the two kvit-coder binaries into the repository root, which is where
# the README and the docs expect to find them, and where kvit-coder-ui looks
# for the kvit-coder child process it spawns for each turn.
#
#   scripts/build.sh                  both binaries, plain development build
#   scripts/build.sh ui               just kvit-coder-ui
#   scripts/build.sh --release        stripped and reproducible, as shipped
#   scripts/build.sh --race coder     one binary with the race detector
#
# Version information is stamped in from git either way; scripts/release.sh is
# still the one to use for a tagged release, since it refuses to build from a
# dirty tree.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

release=0
race=0
targets=()

usage() {
    cat <<'USAGE'
usage: scripts/build.sh [--release] [--race] [coder|ui]...

  --release   optimized build: -s -w -trimpath, no symbol table
  --race      build with the race detector
  coder       build only kvit-coder
  ui          build only kvit-coder-ui

With no target named, both binaries are built into the repository root.
USAGE
}

for arg in "$@"; do
    case "$arg" in
        --release) release=1 ;;
        --race) race=1 ;;
        coder|kvit-coder) targets+=(kvit-coder) ;;
        ui|kvit-coder-ui) targets+=(kvit-coder-ui) ;;
        -h|--help) usage; exit 0 ;;
        *) echo "build.sh: unknown argument: $arg" >&2; usage >&2; exit 2 ;;
    esac
done

if [ ${#targets[@]} -eq 0 ]; then
    targets=(kvit-coder kvit-coder-ui)
fi

version=$(git describe --tags --always 2>/dev/null || echo dev)
commit_hash=$(git rev-parse --short HEAD 2>/dev/null || echo dev)
commit_date=$(git log -1 --format=%cd --date=format:%Y%m%d 2>/dev/null || echo unknown)
build_date=$(date -u +%Y-%m-%d)

if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    version="${version}-dirty"
fi

ldflags="-X main.version=${version} -X main.commitHash=${commit_hash}"
ldflags="${ldflags} -X main.commitDate=${commit_date} -X main.buildDate=${build_date}"

flags=()
if [ "$release" -eq 1 ]; then
    ldflags="-s -w ${ldflags}"
    flags+=(-trimpath)
fi
if [ "$race" -eq 1 ]; then
    flags+=(-race)
fi

echo "building ${targets[*]}  version ${version}  commit ${commit_hash}"

for target in "${targets[@]}"; do
    go build "${flags[@]}" -ldflags="${ldflags}" -o "$ROOT/$target" "./cmd/$target"
done

ls -lh "${targets[@]}"
