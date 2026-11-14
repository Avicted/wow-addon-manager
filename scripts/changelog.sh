#!/bin/sh
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2026 Victor Anderssén
# Maintains CHANGELOG.md from the conventional commit subjects.
#
#   scripts/changelog.sh <version>            add a section for the commits since the last tag
#   scripts/changelog.sh --remove <version>   delete that section and its link
#   scripts/changelog.sh --section <version>  print that section's body (release notes)
#
# CHANGELOG (default CHANGELOG.md) and REPO_URL can be overridden from the
# environment. Run from the repository root.

set -eu

FILE=${CHANGELOG:-CHANGELOG.md}
REPO_URL=${REPO_URL:-https://github.com/Avicted/wow-addon-manager}

usage()
{
	echo "usage: $0 [--remove|--section] <version>" >&2
	exit 2
}

# Prints every line of the file except those for <version>: its section (up to
# the next "## [" heading or the link block) and its "[<version>]:" link.
without_version()
{
	awk -v head="## [$1]" -v link="[$1]:" '
		index($0, head) == 1 { skip = 1; next }
		skip && (/^## \[/ || /^\[[^]]+\]: /) { skip = 0 }
		skip { next }
		index($0, link) == 1 { next }
		{ print }
	' "$FILE"
}

# Collapses runs of blank lines to one (busybox cat has no -s).
squeeze()
{
	awk 'NF || !blank { print } { blank = !NF }'
}

has_section()
{
	grep -qF "## [$1]" "$FILE"
}

case "${1:-}" in
--remove)
	[ $# -eq 2 ] || usage
	has_section "$2" || { echo "changelog: no section for $2" >&2; exit 1; }
	# Collapse the blank line the removed section leaves behind.
	without_version "$2" | squeeze >"$FILE.tmp"
	mv "$FILE.tmp" "$FILE"
	exit 0
	;;
--section)
	[ $# -eq 2 ] || usage
	has_section "$2" || { echo "changelog: no section for $2" >&2; exit 1; }
	awk -v head="## [$2]" '
		index($0, head) == 1 { on = 1; next }
		on && (/^## \[/ || /^\[[^]]+\]: /) { exit }
		on { lines[++n] = $0 }
		END {
			first = 1; while (first <= n && lines[first] == "") first++
			while (n >= first && lines[n] == "") n--
			for (i = first; i <= n; i++) print lines[i]
		}
	' "$FILE"
	exit 0
	;;
-*)
	usage
	;;
esac

[ $# -eq 1 ] || usage
version=$1
[ -f "$FILE" ] || printf '# Changelog\n\n' >"$FILE"
if has_section "$version"; then
	echo "changelog: $FILE already has a section for $version" >&2
	exit 1
fi

prev=$(git describe --tags --abbrev=0 --match 'v[0-9]*' 2>/dev/null || true)
if [ -n "$prev" ]; then
	range="$prev..HEAD"
	link="[$version]: $REPO_URL/compare/$prev...v$version"
else
	range=HEAD
	link="[$version]: $REPO_URL/releases/tag/v$version"
fi

section=$(mktemp)
trap 'rm -f "$section" "$FILE.tmp"' EXIT

{
	printf '## [%s] - %s\n\n' "$version" "$(date +%Y-%m-%d)"
	git log --reverse --no-merges --format='%H %s' "$range" | awk -v url="$REPO_URL" '
		function add(group, text) { body[group] = body[group] "- " text "\n"; found = 1 }
		{
			hash = $1
			subject = substr($0, length(hash) + 2)
			if (subject ~ /^(fixup|squash|amend)! / || subject ~ /^chore\(release\)/)
				next

			type = ""; scope = ""; desc = subject; breaking = 0
			if (match(subject, /^[A-Za-z]+(\([^)]*\))?!?: /)) {
				head = substr(subject, 1, RLENGTH - 2)
				desc = substr(subject, RLENGTH + 1)
				if (substr(head, length(head)) == "!") {
					breaking = 1
					head = substr(head, 1, length(head) - 1)
				}
				type = head
				if ((p = index(head, "(")) > 0) {
					type = substr(head, 1, p - 1)
					scope = substr(head, p + 1, length(head) - p - 1)
				}
			}

			text = (scope != "" ? "**" scope ":** " : "") desc \
				" ([" substr(hash, 1, 7) "](" url "/commit/" hash "))"
			if (breaking)        add("Breaking changes", text)
			else if (type == "feat") add("Features", text)
			else if (type == "fix")  add("Fixes", text)
			else if (type == "perf") add("Performance", text)
			else if (type == "docs") add("Documentation", text)
			else                 add("Other changes", text)
		}
		END {
			if (!found) { print "No notable changes.\n"; exit }
			n = split("Breaking changes,Features,Fixes,Performance,Documentation,Other changes", order, ",")
			for (i = 1; i <= n; i++)
				if (order[i] in body)
					printf "### %s\n\n%s\n", order[i], body[order[i]]
		}
	'
} >"$section"

# The section goes above the newest existing one (or the link block, or the
# end), and its link heads the link block.
awk -v sectionfile="$section" -v link="$link" '
	function emit_section(  line) {
		while ((getline line < sectionfile) > 0) print line
		close(sectionfile); placed = 1
	}
	!placed && /^## \[/ { emit_section() }
	/^\[[^]]+\]: / && !linked {
		if (!placed) emit_section()
		print link; linked = 1
	}
	{ print }
	END {
		if (!placed) { print ""; emit_section() }
		if (!linked) print link
	}
' "$FILE" | squeeze >"$FILE.tmp"
mv "$FILE.tmp" "$FILE"
echo "changelog: added $version ($range) to $FILE"
