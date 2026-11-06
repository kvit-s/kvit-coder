#!/bin/bash
set -euo pipefail

# Tags the next release so the Release workflow builds it.
#
# The Release workflow (.github/workflows/release.yml) runs on pushed tags
# matching v*, so cutting a release is: pick the next number, tag HEAD,
# push the tag. This script does the bumping, the tagging and the push
# in one command:
#
#   scripts/tag-release.sh --patch -m "fix ..."
#   scripts/tag-release.sh --minor -m "new feature ..."
#   scripts/tag-release.sh --major -m "breaking change ..."
#   scripts/tag-release.sh --set v1.2.3 -m "anniversary release"
#
# Numbers follow semver: --patch bumps 0.7.0 to 0.7.1, --minor bumps it to
# 0.8.0 (patch reset to zero), --major bumps it to 1.0.0. The tag is
# annotated ("v0.7.1: <message>"), matching the existing tags, and the push
# is of the tag only, which is what CI listens for.
#
# The script refuses a dirty tree (like scripts/release.sh), refuses when
# HEAD already carries a version tag, shows the commits since the previous
# tag, runs go test ./... as a sanity check, then creates and pushes the tag.
#
# It never asks anything, because it is run from Kvit Works' command buttons,
# where nobody can answer: running it is the decision to release. Use
# --dry-run to see what it would do. Git and ssh are told not to prompt
# either, so a missing credential fails with an error instead of waiting, and
# a failed push removes the local tag so the command can simply be run again.

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

export GIT_TERMINAL_PROMPT=0
export GIT_SSH_COMMAND="${GIT_SSH_COMMAND:-ssh} -o BatchMode=yes"

kind=""
explicit=""
message=""
push=1
verify=1
fetch_tags=1

usage() {
    cat <<'USAGE'
usage: scripts/tag-release.sh (--patch|--minor|--major|--set VERSION) [options]

  --patch          bump the patch number: 0.7.0 -> 0.7.1
  --minor          bump the minor number: 0.7.0 -> 0.8.0
  --major          bump the major number: 0.7.0 -> 1.0.0
  --set VERSION    use this version instead (v1.2.3 or 1.2.3)

  -m, --message TEXT   tag summary, used as "vX.Y.Z: TEXT"
                       (default: first line of commits since last tag,
                       or just vX.Y.Z when there is nothing to summarize)
  --no-push        create the tag locally without pushing it
  --no-verify      skip go test ./... before tagging
  --no-fetch       skip git fetch origin --tags before computing the number
  -h, --help       show this help

examples:
  scripts/tag-release.sh --patch -m "fix picker Space toggle"
  scripts/tag-release.sh --minor -m "automatic context compaction"
  scripts/tag-release.sh --patch --dry-run
USAGE
}

dry_run=0

while [ $# -gt 0 ]; do
    case "$1" in
        --patch|--minor|--major)
            [ -z "$kind" ] || { echo "tag-release.sh: only one of --patch/--minor/--major/--set" >&2; exit 2; }
            [ -z "$explicit" ] || { echo "tag-release.sh: only one of --patch/--minor/--major/--set" >&2; exit 2; }
            kind="${1#--}" ;;
        --set)
            [ -z "$kind" ] || { echo "tag-release.sh: only one of --patch/--minor/--major/--set" >&2; exit 2; }
            [ -z "$explicit" ] || { echo "tag-release.sh: only one of --patch/--minor/--major/--set" >&2; exit 2; }
            shift
            [ $# -gt 0 ] || { echo "tag-release.sh: --set needs a version" >&2; exit 2; }
            explicit="$1" ;;
        --set=*) explicit="${1#--set=}" ;;
        -m|--message)
            shift
            [ $# -gt 0 ] || { echo "tag-release.sh: --message needs text" >&2; exit 2; }
            message="$1" ;;
        --message=*) message="${1#--message=}" ;;
        --no-push) push=0 ;;
        --no-verify) verify=0 ;;
        --no-fetch) fetch_tags=0 ;;
        --dry-run) dry_run=1 ;;
        -h|--help) usage; exit 0 ;;
        *) echo "tag-release.sh: unknown argument: $1" >&2; usage >&2; exit 2 ;;
    esac
    shift
done

if [ -z "$kind" ] && [ -z "$explicit" ]; then
    echo "tag-release.sh: pick a bump: --patch, --minor, --major or --set VERSION" >&2
    usage >&2
    exit 2
fi

if [ "$dry_run" -eq 0 ] && [ -n "$(git status --porcelain 2>/dev/null)" ]; then
    echo "tag-release.sh: git working tree is not clean" >&2
    echo "commit or stash your changes before tagging a release" >&2
    git status --short >&2
    exit 1
fi

if [ "$fetch_tags" -eq 1 ]; then
    git fetch origin --tags --quiet 2>/dev/null || echo "tag-release.sh: warning: could not fetch tags, using local ones" >&2
fi

latest=$(git tag --list 'v[0-9]*' --sort=-v:refname | head -n 1 || true)

next=""
if [ -n "$explicit" ]; then
    explicit="${explicit#v}"
    if [[ ! "$explicit" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "tag-release.sh: --set needs X.Y.Z, got: $explicit" >&2
        exit 2
    fi
    next="v$explicit"
else
    base="${latest#v}"
    [ -n "$base" ] || base="0.0.0"
    if [[ ! "$base" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
        echo "tag-release.sh: latest tag is not X.Y.Z, use --set: $latest" >&2
        exit 1
    fi
    major="${base%%.*}"
    rest="${base#*.}"
    minor="${rest%%.*}"
    patch="${rest#*.}"
    case "$kind" in
        patch) patch=$((patch + 1)) ;;
        minor) minor=$((minor + 1)); patch=0 ;;
        major) major=$((major + 1)); minor=0; patch=0 ;;
    esac
    next="v$major.$minor.$patch"
fi

if git rev-parse -q --verify "refs/tags/$next" >/dev/null; then
    echo "tag-release.sh: tag already exists: $next" >&2
    exit 1
fi

if exact=$(git describe --tags --exact-match HEAD 2>/dev/null); then
    echo "tag-release.sh: HEAD already tagged as $exact, nothing new to release" >&2
    exit 1
fi

if [ -z "$message" ]; then
    if [ -n "$latest" ]; then
        message=$(git log "$latest..HEAD" --format=%s --no-merges | head -n 1 || true)
    fi
    [ -n "$message" ] || message="$next"
fi
tagline="$next: $message"

echo "latest tag:  ${latest:-(none)}"
echo "next tag:    $next"
echo "annotated:   $tagline"
echo "head:        $(git rev-parse --short HEAD) $(git log -1 --format=%s)"
if [ -n "$latest" ]; then
    echo "commits since $latest:"
    git log "$latest..HEAD" --format='  %h %s' --no-merges || true
fi

if [ "$verify" -eq 1 ] && [ "$dry_run" -eq 0 ]; then
    echo "running go test ./... ..."
    go test ./...
fi

if [ "$dry_run" -eq 1 ]; then
    echo "dry run: would create tag $next and $([ "$push" -eq 1 ] && echo "push it to origin" || echo "leave it local")"
    exit 0
fi

git tag -a "$next" -m "$tagline"
if [ "$push" -eq 1 ]; then
    if ! git push origin "$next"; then
        git tag -d "$next" >/dev/null
        echo "tag-release.sh: push failed, removed the local tag $next; nothing was released" >&2
        exit 1
    fi
    echo "pushed $next; CI builds the release:"
    echo "  https://github.com/kvit-s/kvit-coder/releases/tag/$next"
else
    echo "created $next locally; push it with: git push origin $next"
fi
