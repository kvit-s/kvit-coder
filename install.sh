#!/bin/sh
# Install kvit-coder from a published release.
#
#   curl -fsSL https://raw.githubusercontent.com/kvit-s/kvit-coder/main/install.sh | sh
#
# It works out which build fits this machine, downloads it from the GitHub
# release, checks it against the release's checksums file, and puts both
# binaries in ~/.local/bin under four names: kc and kvit-coder for the headless
# agent, kcu and kvit-coder-ui for the interactive front end. If there is no
# configuration yet it leaves one at ~/.kvit-coder/config.yaml to edit.
#
# For native Windows (outside WSL) use the installer on the release page, or
# install.ps1, which installs the Windows zip release the same way.
#
# Environment:
#   KVIT_VERSION   a release tag to install instead of the latest (e.g. v0.1.0)
#   KVIT_BIN_DIR   where to put the binaries (default ~/.local/bin)
#
# This installs a release. To work on kvit-coder itself, clone the repository
# and run scripts/install.sh, which links the binaries you build.

set -eu

REPO="kvit-s/kvit-coder"
BIN_DIR="${KVIT_BIN_DIR:-$HOME/.local/bin}"
CONFIG_DIR="$HOME/.kvit-coder"

say() { printf '%s\n' "$*"; }
die() { printf 'install: %s\n' "$*" >&2; exit 1; }

need() {
	command -v "$1" >/dev/null 2>&1 || die "this script needs $1, which is not on the PATH"
}

# --- which build fits this machine -------------------------------------
detect_platform() {
	os=$(uname -s)
	arch=$(uname -m)

	case "$os" in
		Linux) os=linux ;;
		Darwin) os=darwin ;;
		*)
			die "no build for $os. Linux is what this is tested on, macOS is built but untested, and Windows is not supported — under Windows, run it inside WSL."
			;;
	esac

	case "$arch" in
		x86_64 | amd64) arch=amd64 ;;
		aarch64 | arm64) arch=arm64 ;;
		*) die "no build for $arch (amd64 and arm64 only)" ;;
	esac

	PLATFORM="${os}_${arch}"
}

# --- which release -----------------------------------------------------
latest_version() {
	# The redirect on /releases/latest names the tag, which avoids parsing JSON.
	url=$(curl -fsSLI -o /dev/null -w '%{url_effective}' "https://github.com/$REPO/releases/latest") ||
		die "cannot reach GitHub to ask for the latest release"
	tag=${url##*/}
	case "$tag" in
		v*) printf '%s\n' "$tag" ;;
		*) die "no published release found for $REPO" ;;
	esac
}

main() {
	need curl
	need tar

	if command -v sha256sum >/dev/null 2>&1; then
		SHA="sha256sum"
	elif command -v shasum >/dev/null 2>&1; then
		SHA="shasum -a 256"
	else
		die "this script needs sha256sum or shasum to check the download"
	fi

	detect_platform
	version="${KVIT_VERSION:-$(latest_version)}"
	number=${version#v}
	archive="kvit-coder_${number}_${PLATFORM}.tar.gz"
	base="https://github.com/$REPO/releases/download/$version"

	tmp=$(mktemp -d) || die "cannot make a temporary directory"
	trap 'rm -rf "$tmp"' EXIT INT TERM

	say "kvit-coder $version for $PLATFORM"
	curl -fsSL "$base/$archive" -o "$tmp/$archive" ||
		die "cannot download $archive — check that $version has a build for $PLATFORM"
	curl -fsSL "$base/checksums.txt" -o "$tmp/checksums.txt" ||
		die "cannot download the checksums for $version"

	# Check this one file rather than the whole list, which names archives that
	# were not downloaded.
	want=$(grep " $archive\$" "$tmp/checksums.txt" | cut -d' ' -f1) ||
		die "the release's checksums do not mention $archive"
	[ -n "$want" ] || die "the release's checksums do not mention $archive"
	got=$(cd "$tmp" && $SHA "$archive" | cut -d' ' -f1)
	[ "$want" = "$got" ] || die "the download does not match its checksum; not installing"

	tar -xzf "$tmp/$archive" -C "$tmp" || die "cannot unpack $archive"

	mkdir -p "$BIN_DIR"
	for binary in kvit-coder kvit-coder-ui; do
		[ -f "$tmp/$binary" ] || die "$archive has no $binary in it"
		install -m 0755 "$tmp/$binary" "$BIN_DIR/$binary" 2>/dev/null ||
			{ cp "$tmp/$binary" "$BIN_DIR/$binary" && chmod 0755 "$BIN_DIR/$binary"; }
	done
	ln -sf "$BIN_DIR/kvit-coder" "$BIN_DIR/kc"
	ln -sf "$BIN_DIR/kvit-coder-ui" "$BIN_DIR/kcu"

	# A first install has nothing to run against, so leave a configuration to
	# edit rather than an error message about not finding one.
	fresh=no
	if [ ! -f "$CONFIG_DIR/config.yaml" ] && [ -f "$tmp/config.example.yaml" ]; then
		mkdir -p "$CONFIG_DIR"
		cp "$tmp/config.example.yaml" "$CONFIG_DIR/config.yaml"
		fresh=yes
	fi

	say "installed kvit-coder, kvit-coder-ui, kc and kcu in $BIN_DIR"

	case ":$PATH:" in
		*":$BIN_DIR:"*) ;;
		*) say "" ; say "$BIN_DIR is not on your PATH. Add it:" ; say "  export PATH=\"\$PATH:$BIN_DIR\"" ;;
	esac

	if [ "$fresh" = yes ]; then
		say ""
		say "A starting configuration is at $CONFIG_DIR/config.yaml."
		say "Run kcu in any directory you want to work in; it opens :setup to"
		say "choose a model provider, take its key and pick the models."
	else
		say ""
		say "Run kcu in any directory you want to work in."
	fi
}

main "$@"
