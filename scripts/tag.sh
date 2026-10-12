#!/usr/bin/env bash
set -euo pipefail

if [ "$#" -ne 1 ]; then
    printf 'Usage: make tag major|minor|patch\n' >&2
    exit 1
fi
case "$1" in
    major|minor|patch) ;;
    *) printf 'Invalid version bump: %s (expected major, minor, or patch)\n' "$1" >&2; exit 1 ;;
esac

git rev-parse --show-toplevel >/dev/null
git rev-parse --verify HEAD >/dev/null
if [ -n "$(git status --porcelain)" ]; then
    printf 'Cannot tag a dirty worktree. Commit or stash changes first.\n' >&2
    exit 1
fi

tags=$(git tag --list 'v*' --sort=-version:refname)
latest=v0.0.0
while IFS= read -r tag; do
    if [[ "$tag" =~ ^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$ ]]; then
        latest=$tag
        break
    fi
done <<< "$tags"

# Increment decimal text without limiting SemVer components to shell integers.
increment() {
    local number=$1 suffix="" digit
    while [ -n "$number" ]; do
        digit=${number: -1}
        number=${number%?}
        if [ "$digit" != 9 ]; then
            printf '%s%s%s' "$number" "$((digit + 1))" "$suffix"
            return
        fi
        suffix="0$suffix"
    done
    printf '1%s' "$suffix"
}

version=${latest#v}
major=${version%%.*}
version=${version#*.}
minor=${version%%.*}
patch=${version#*.}
case "$1" in
    major) major=$(increment "$major"); minor=0; patch=0 ;;
    minor) minor=$(increment "$minor"); patch=0 ;;
    patch) patch=$(increment "$patch") ;;
esac
next="v$major.$minor.$patch"
git tag -a "$next" -m "Release $next"
printf 'Created %s at HEAD (previous version: %s).\n' "$next" "$latest"
printf 'To publish the release, run: git push origin %s\n' "$next"
