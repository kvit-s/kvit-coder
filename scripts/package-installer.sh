#!/bin/bash
set -euo pipefail

# Builds the Windows installers, one per architecture:
#
#   dist/installer/kvit-coder_<number>_windows_<arch>_setup.exe
#
# scripts/windows-installer.iss says what an installer does: a per-user
# install into %LOCALAPPDATA%\Programs\kvit-coder, that folder on the user's
# PATH, and kc.exe and kcu.exe beside the two programs.
#
#   scripts/package-installer.sh [--arch amd64|arm64] [version]
#
# With no version, git describe names it (or "dev" when tags are missing, as
# on a shallow CI checkout); the release job passes the tag. With no --arch,
# both architectures are built.
#
# The installer is compiled by Inno Setup 6, a Windows program, so this runs
# on Windows under Git Bash (the release job in .github/workflows/release.yml)
# or in WSL, which starts ISCC.exe directly. ISCC.exe is looked for at
# $KVIT_ISCC, then on the PATH, then in the two Program Files folders, then in
# %LOCALAPPDATA%\Programs\Inno Setup 6, where Inno's installer puts a per-user
# install. Inno Setup: https://jrsoftware.org/isdl.php, or
# winget install JRSoftware.InnoSetup.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

usage() {
    cat <<'USAGE'
usage: scripts/package-installer.sh [--arch amd64|arm64] [version]

  --arch    build only this architecture (default: amd64 and arm64)
  version   release tag for file names and the version stamp
            (default: git describe --tags --always, or dev)
USAGE
}

archs=()
version=""
while [ $# -gt 0 ]; do
    case "$1" in
        --arch) shift; archs+=("${1:?--arch needs amd64 or arm64}") ;;
        -h|--help) usage; exit 0 ;;
        -*) echo "package-installer.sh: unknown argument: $1" >&2; usage >&2; exit 2 ;;
        *) version=$1 ;;
    esac
    shift
done
[ ${#archs[@]} -gt 0 ] || archs=(amd64 arm64)
for goarch in "${archs[@]}"; do
    case "$goarch" in
        amd64|arm64) ;;
        *) echo "package-installer.sh: no Windows build for $goarch (amd64 and arm64 only)" >&2; exit 2 ;;
    esac
done

# Windows paths for ISCC's arguments: cygpath under Git Bash, wslpath in WSL.
if command -v cygpath >/dev/null 2>&1; then
    win_path() { cygpath -w "$1"; }
    unix_path() { cygpath -u "$1"; }
    win_env() { printenv "$1" || true; }
elif command -v wslpath >/dev/null 2>&1; then
    win_path() { wslpath -w "$1"; }
    unix_path() { wslpath -u "$1"; }
    # cmd.exe warns when started from a Linux folder, so it starts from C:.
    win_env() { (cd /mnt/c && cmd.exe /d /c "echo %$1%") | tr -d '\r'; }
else
    echo "package-installer.sh: Inno Setup is a Windows program; run this on Windows (Git Bash) or in WSL" >&2
    exit 1
fi

find_iscc() {
    local c
    if [ -n "${KVIT_ISCC:-}" ]; then
        c=$KVIT_ISCC
        [[ $c == [A-Za-z]:* ]] && c=$(unix_path "$c")
        [ -f "$c" ] || { echo "package-installer.sh: KVIT_ISCC=$KVIT_ISCC does not exist" >&2; return 1; }
        echo "$c"
        return 0
    fi
    c=$(command -v ISCC.exe || command -v iscc.exe || true)
    if [ -n "$c" ]; then
        echo "$c"
        return 0
    fi
    for c in 'C:\Program Files (x86)\Inno Setup 6\ISCC.exe' \
             'C:\Program Files\Inno Setup 6\ISCC.exe' \
             "$(win_env LOCALAPPDATA)\\Programs\\Inno Setup 6\\ISCC.exe"; do
        c=$(unix_path "$c")
        if [ -f "$c" ]; then
            echo "$c"
            return 0
        fi
    done
    echo "package-installer.sh: Inno Setup 6 (ISCC.exe) was not found; install it from" \
         "https://jrsoftware.org/isdl.php (or: winget install JRSoftware.InnoSetup)," \
         "or set KVIT_ISCC to its ISCC.exe" >&2
    return 1
}

command -v go >/dev/null 2>&1 || { echo "package-installer.sh: needs go on the PATH" >&2; exit 1; }
ISCC=$(find_iscc)

[ -n "$version" ] || version=$(git describe --tags --always 2>/dev/null || echo dev)
number="${version#v}"  # file names follow goreleaser: 0.3.0, not v0.3.0
# The installer's own file version takes numbers only: 0.6.0 for 0.6.0-rc1,
# and 0.0.0 for a build that is not on a tag.
numeric=$(sed -nE 's/^([0-9]+(\.[0-9]+){0,3}).*/\1/p' <<< "$number")
[ -n "$numeric" ] || numeric=0.0.0
commit_hash=$(git rev-parse --short HEAD 2>/dev/null || echo dev)
commit_date=$(git log -1 --format=%cd --date=format:%Y%m%d 2>/dev/null || echo unknown)
build_date=$(date -u +%Y-%m-%d)

if [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    version="${version}-dirty"
fi

ldflags="-s -w -X main.version=${version} -X main.commitHash=${commit_hash}"
ldflags="${ldflags} -X main.commitDate=${commit_date} -X main.buildDate=${build_date}"

out="$ROOT/dist/installer"
mkdir -p "$out"
rm -f "$out"/kvit-coder_*_windows_*_setup.exe

echo "Inno Setup: $(win_path "$ISCC")"
for goarch in "${archs[@]}"; do
    setup="kvit-coder_${number}_windows_${goarch}_setup.exe"
    stage=$(mktemp -d "$ROOT/dist/installer-stage.XXXXXX")
    trap 'rm -rf "$stage"' EXIT
    echo "building windows/$goarch version ${version} commit ${commit_hash}"
    CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags="$ldflags" \
        -o "$stage/kvit-coder.exe" ./cmd/kvit-coder
    CGO_ENABLED=0 GOOS=windows GOARCH="$goarch" go build -trimpath -ldflags="$ldflags" \
        -o "$stage/kvit-coder-ui.exe" ./cmd/kvit-coder-ui
    cp "$ROOT/config.example.yaml" "$ROOT/README.md" "$ROOT/LICENSE" "$stage/"

    # Git Bash rewrites arguments that look like unix paths before starting a
    # Windows program, which would turn /Q into Q:/; these turn that off.
    MSYS_NO_PATHCONV=1 MSYS2_ARG_CONV_EXCL='*' "$ISCC" /Q \
        "/DKvitVersion=$number" \
        "/DKvitVersionNumeric=$numeric" \
        "/DKvitArch=$goarch" \
        "/DStageDir=$(win_path "$stage")" \
        "/DOutputDir=$(win_path "$out")" \
        "$(win_path "$ROOT/scripts/windows-installer.iss")" | tr -d '\r'
    [ -f "$out/$setup" ] || { echo "package-installer.sh: Inno Setup did not write $out/$setup" >&2; exit 1; }
    trap - EXIT
    rm -rf "$stage"
done

echo "installers:"
ls -lh "$out"
if command -v sha256sum >/dev/null 2>&1; then
    (cd "$out" && sha256sum ./*.exe)
fi
