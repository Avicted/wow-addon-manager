#!/bin/sh
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2026 Victor Anderssén
# Checks that every version bump in a range of commits was made by `make release`.
#
#   scripts/check-version-bumps.sh <range>    e.g. v1.0.0..HEAD, or HEAD for all history
#
# A commit that changes VERSION must:
#   - have the subject `chore(release): <new version>`,
#   - change only VERSION and CHANGELOG.md,
#   - step the version by one major, minor or patch bump,
#   - carry a CHANGELOG.md section `## [<new version>]`.
# A `chore(release):` commit must change VERSION. The commit that creates VERSION is skipped.
# Run from the repository root.

set -eu

[ $# -eq 1 ] || { echo "usage: $0 <range>" >&2; exit 2; }

# Prints the versions a bump from $1 may lead to: next major, minor and patch.
next_versions()
{
	echo "$1" | awk -F. '{
		printf "%d.0.0\n%d.%d.0\n%d.%d.%d\n", $1 + 1, $1, $2 + 1, $1, $2, $3 + 1 }'
}

errors=0
fail()
{
	echo "version-check: $(git log -1 --format='%h %s' "$1"): $2" >&2
	errors=$((errors + 1))
}

checked=0
for c in $(git rev-list --reverse --no-merges "$1"); do
	subject=$(git log -1 --format=%s "$c")
	files=$(git diff-tree --root --no-commit-id --name-only -r "$c")
	if ! echo "$files" | grep -qx VERSION; then
		case "$subject" in
		"chore(release)"*) fail "$c" "a release commit must change VERSION (use make release)" ;;
		esac
		continue
	fi
	if ! git cat-file -e "$c^" 2>/dev/null; then
		continue # the first commit, or the edge of a shallow clone
	fi
	if ! old=$(git show "$c^:VERSION" 2>/dev/null); then
		continue # this commit creates VERSION
	fi
	new=$(git show "$c:VERSION")
	checked=$((checked + 1))

	[ "$subject" = "chore(release): $new" ] ||
		fail "$c" "changes VERSION to $new; bump versions with make release (subject must be 'chore(release): $new')"
	[ "$(echo "$files" | LC_ALL=C sort | tr '\n' ' ')" = "CHANGELOG.md VERSION " ] ||
		fail "$c" "a release commit changes only VERSION and CHANGELOG.md, not: $(echo "$files" | tr '\n' ' ')"
	if echo "$old" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$' && echo "$new" | grep -qE '^[0-9]+\.[0-9]+\.[0-9]+$'; then
		next_versions "$old" | grep -qx "$new" ||
			fail "$c" "$old -> $new is not one major, minor or patch step"
	else
		fail "$c" "$old -> $new: versions must be MAJOR.MINOR.PATCH"
	fi
	git show "$c:CHANGELOG.md" 2>/dev/null | grep -qF "## [$new]" ||
		fail "$c" "CHANGELOG.md has no section for $new"
done

[ "$errors" -eq 0 ] || exit 1
echo "version-check: OK ($checked version bumps in $1)"
