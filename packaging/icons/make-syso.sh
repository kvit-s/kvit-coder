#!/usr/bin/env bash
# Writes the Windows resources of kvit-coder.exe and kvit-coder-ui.exe:
# rsrc_windows_amd64.syso and rsrc_windows_arm64.syso in cmd/kvit-coder and
# in cmd/kvit-coder-ui. Each holds the program's icon (kvit-coder.ico, which
# `go -C packaging/icons run .` draws from kvit-coder.svg) and its version
# information: the description Task Manager shows for the process, and what
# Explorer shows under Properties > Details.
#
#   packaging/icons/make-syso.sh [version]
#
# The go command links a .syso file in a package's folder into every Windows
# build of that package for its architecture. The committed files say
# version "dev", which is what a development build shows. The release builds
# (.goreleaser.yml, scripts/package-portable.sh, scripts/package-installer.sh)
# run this with the release version before they build, and again with no
# version afterwards, which writes the committed files back byte for byte.
#
# The tool is go-winres (github.com/tc-hib/go-winres), run with `go run` at a
# pinned version, as Kvit Works runs it, so it needs nothing installed and
# leaves go.mod alone. It writes no application manifest: a manifest changes
# how Windows runs the program (its code page, long paths, scaling), which is
# a decision of its own.
set -euo pipefail
cd "$(dirname "$0")/../.."

GO_WINRES=github.com/tc-hib/go-winres@v0.3.3

version=${1:-dev}
version=${version#v}  # 0.6.0, as goreleaser names a release, not v0.6.0

# resources writes one program's two .syso files; the description is what
# Task Manager lists the process as.
resources() {
    local program=$1 description=$2
    go run "$GO_WINRES" simply \
        --arch amd64,arm64 \
        --out "cmd/$program/rsrc" \
        --manifest none \
        --icon packaging/icons/kvit-coder.ico \
        --product-name kvit-coder \
        --file-description "$description" \
        --original-filename "$program.exe" \
        --copyright "Copyright (c) 2025 kvit-s, MIT License" \
        --product-version "$version" \
        --file-version "$version"
}

resources kvit-coder-ui "kvit-coder"
resources kvit-coder "kvit-coder agent"
echo "wrote cmd/kvit-coder/rsrc_windows_{amd64,arm64}.syso and the same in cmd/kvit-coder-ui (version $version)"
