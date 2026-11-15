#!/bin/sh
# SPDX-License-Identifier: MIT
# SPDX-FileCopyrightText: 2026 Victor Anderssén
# Prints the shields.io endpoint JSON for the coverage badge, read from the `total:` line of
# the per-function summary `make cover` writes.
#
#   scripts/coverage-badge.sh build/coverage/coverage.txt > coverage.json
set -eu

[ $# -eq 1 ] || { echo "usage: $0 <coverage.txt>" >&2; exit 2; }

pct=$(awk '$1 == "total:" { sub(/%$/, "", $NF); print $NF }' "$1")
case "$pct" in
'' | *[!0-9.]*) echo "coverage-badge: no total: line in $1" >&2; exit 1 ;;
esac

color=$(awk -v p="$pct" 'BEGIN { print (p >= 80 ? "brightgreen" : p >= 60 ? "yellow" : "red") }')
printf '{"schemaVersion":1,"label":"coverage","message":"%s%%","color":"%s"}\n' "$pct" "$color"
