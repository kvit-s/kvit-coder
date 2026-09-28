#!/bin/bash
set -euo pipefail

# Puts kvit-coder on the PATH so it can be started in any directory.
#
# It builds both binaries and then links them from a directory on the PATH
# (~/.local/bin by default) back into this checkout:
#
#   kc              -> kvit-coder      the headless agent, short name
#   kcu             -> kvit-coder-ui   the interactive front end, short name
#   kvit-coder      -> kvit-coder
#   kvit-coder-ui   -> kvit-coder-ui
#
# They are symlinks rather than copies, so scripts/build.sh is all that is
# needed after a change; there is nothing to install again.
#
# On native Windows this symlink farm needs privilege; prefer .exe copies on
# the PATH instead (install.ps1 does this for releases).
#
#   scripts/install.sh                    build, then link into ~/.local/bin
#   scripts/install.sh --no-build         link what is already built
#   scripts/install.sh --bin-dir ~/bin    link somewhere else
#   scripts/install.sh --uninstall        remove the links this script made
#
# The directory the command runs in becomes the workspace, because
# workspace.root is ".". The configuration is found at the first of
# $KVIT_CODER_CONFIG, ./config.yaml, ~/.kvit-coder/config.yaml, and config.yaml
# beside the binary with symlinks resolved -- so a config.yaml left in this
# checkout is picked up everywhere the symlinks are used, which is convenient
# while working on the program and surprising otherwise. A fresh checkout has
# none: copy config.example.yaml to ~/.kvit-coder/config.yaml.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

BIN_DIR=${KVIT_BIN_DIR:-$HOME/.local/bin}
build=1
uninstall=0
LINKS=(kc kcu kvit-coder-ui kvit-coder)

usage() {
    cat <<'USAGE'
usage: scripts/install.sh [--no-build] [--uninstall] [--bin-dir <dir>]

  --no-build        skip scripts/build.sh and link the existing binaries
  --uninstall       remove kc, kcu, kvit-coder-ui and kvit-coder from the bin dir
  --bin-dir <dir>   where to put the links (default ~/.local/bin, or $KVIT_BIN_DIR)
USAGE
}

while [ $# -gt 0 ]; do
    case "$1" in
        --no-build) build=0 ;;
        --uninstall) uninstall=1 ;;
        --bin-dir) shift; BIN_DIR=${1:?--bin-dir needs a directory} ;;
        -h|--help) usage; exit 0 ;;
        *) echo "install.sh: unknown argument: $1" >&2; usage >&2; exit 2 ;;
    esac
    shift
done

if [ "$uninstall" -eq 1 ]; then
    for name in "${LINKS[@]}"; do
        link="$BIN_DIR/$name"
        target=$(readlink "$link" 2>/dev/null || true)
        case "$target" in
            "$ROOT"/*) rm -f "$link"; echo "removed $link" ;;
            "") [ -e "$link" ] && echo "left $link alone: not a symlink" ;;
            *) echo "left $link alone: it points at $target" ;;
        esac
    done
    exit 0
fi

if [ "$build" -eq 1 ]; then
    "$ROOT/scripts/build.sh"
fi

for binary in kvit-coder kvit-coder-ui; do
    if [ ! -x "$ROOT/$binary" ]; then
        echo "install.sh: $ROOT/$binary is missing; run scripts/build.sh" >&2
        exit 1
    fi
done

mkdir -p "$BIN_DIR"

link_one() {
    local name=$1 target=$2 link="$BIN_DIR/$1"
    if [ -e "$link" ] && [ ! -L "$link" ]; then
        echo "install.sh: $link exists and is not a symlink; leaving it alone" >&2
        return 1
    fi
    ln -sfn "$target" "$link"
    echo "$link -> $target"
}

link_one kvit-coder "$ROOT/kvit-coder"
link_one kvit-coder-ui "$ROOT/kvit-coder-ui"
link_one kc "$ROOT/kvit-coder"
link_one kcu "$ROOT/kvit-coder-ui"

case ":$PATH:" in
    *":$BIN_DIR:"*) ;;
    *) echo
       echo "note: $BIN_DIR is not on your PATH. Add this to ~/.bashrc:"
       echo "    export PATH=\"$BIN_DIR:\$PATH\""
       ;;
esac
