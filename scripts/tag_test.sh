#!/usr/bin/env bash
set -euo pipefail

project=$(cd "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -r "$work"' EXIT
export GIT_CONFIG_NOSYSTEM=1
export GIT_CONFIG_GLOBAL=/dev/null

fixture() {
    local name=$1
    mkdir "$work/$name"
    cd "$work/$name"
    git init -q
    git config user.name "Tag Test"
    git config user.email "tag-test@example.invalid"
    git config commit.gpgsign false
    git config tag.gpgsign false
    mkdir scripts
    cp "$project/Makefile" Makefile
    cp "$project/scripts/tag.sh" scripts/tag.sh
    git add Makefile scripts/tag.sh
    git commit -qm "Test fixture"
}

assert_tag() {
    local tag=$1
    git rev-parse --verify "refs/tags/$tag" >/dev/null
    [ "$(git cat-file -t "refs/tags/$tag")" = tag ]
    [ "$(git rev-parse "$tag^{}")" = "$(git rev-parse HEAD)" ]
}

reject() {
    local before
    before=$(git tag --list)
    if make --no-print-directory "$@" >"$work/rejected.log" 2>&1; then
        printf 'Expected failure: make %s\n' "$*" >&2
        exit 1
    fi
    [ "$before" = "$(git tag --list)" ]
}

for bump in major minor patch; do
    fixture "initial-$bump"
    make --no-print-directory tag "$bump"
    case "$bump" in
        major) assert_tag v1.0.0 ;;
        minor) assert_tag v0.1.0 ;;
        patch) assert_tag v0.0.1 ;;
    esac
done

fixture bumps
git tag v0.1.1
make --no-print-directory -j4 tag minor
assert_tag v0.2.0
make --no-print-directory tag patch
assert_tag v0.2.1
make --no-print-directory tag major
assert_tag v1.0.0
git tag v2.9.9
git tag v2.10.0
git tag v1.99.99
git tag v99.0.0-rc.1
git tag v100.0.0+build
git tag v.999.0.0
git tag v999.01.0
make --no-print-directory tag patch
assert_tag v2.10.1
git tag v3.9.99
make --no-print-directory tag patch
assert_tag v3.9.100
git tag v4.0.99999999999999999999
make --no-print-directory tag patch
assert_tag v4.0.100000000000000000000

reject tag
reject tag minor patch
reject tag invalid
reject minor
reject -j4 tag minor build
printf 'dirty\n' >> Makefile
reject tag patch
git restore Makefile
printf 'untracked\n' > untracked.txt
reject tag patch

git -C "$work" init -q empty-repo
cd "$work/empty-repo"
if bash "$project/scripts/tag.sh" patch >"$work/empty.log" 2>&1; then
    printf 'Expected failure for repository without a commit.\n' >&2
    exit 1
fi
printf 'Version tagging tests passed.\n'
