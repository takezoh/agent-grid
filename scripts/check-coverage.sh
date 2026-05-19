#!/usr/bin/env bash
# Run `go test -cover` and verify every package meets its floor from
# scripts/coverage-floors.txt. Exits non-zero if any package regresses
# below its floor or a covered package is missing from the floor file.
#
# Floors are deliberately a few points below the latest measurement so
# expected variance doesn't break the build; raise them when coverage
# gains stick. Tier-level *targets* live in docs/testing.md.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FLOORS_FILE="$REPO_ROOT/scripts/coverage-floors.txt"
TEST_LOG="${TMPDIR:-/tmp}/coverage-check-$$.log"
trap 'rm -f "$TEST_LOG"' EXIT

cd "$REPO_ROOT/src"

# TMPDIR=/tmp so Unix-socket tests can listen; the sandbox blocks the
# default TMPDIR.
TMPDIR=/tmp go test -short -timeout 300s -cover ./... 2>&1 | tee "$TEST_LOG" >&2

declare -A floors
while IFS= read -r line; do
    line="${line%%#*}"
    [[ -z "${line// }" ]] && continue
    read -r pkg pct _ <<<"$line"
    floors[$pkg]=$pct
done <"$FLOORS_FILE"

violations=0
unknown=0
checked=0

while IFS= read -r line; do
    if [[ $line =~ ^(ok|---FAIL).*"github.com/takezoh/agent-roost"[^[:space:]]*.*coverage:\ ([0-9.]+)% ]]; then
        # parse "ok  <pkg> <time>  coverage: NN.N% of statements"
        pkg=$(awk '{print $2}' <<<"$line")
        pct="${BASH_REMATCH[2]}"
    elif [[ $line =~ ^[[:space:]]+(github.com/takezoh/agent-roost[^[:space:]]*).*coverage:\ ([0-9.]+)% ]]; then
        # "<tab>github.com/.../pkg<tab><tab>coverage: 0.0% of statements"
        pkg="${BASH_REMATCH[1]}"
        pct="${BASH_REMATCH[2]}"
    else
        continue
    fi
    checked=$((checked + 1))
    floor="${floors[$pkg]:-}"
    if [[ -z $floor ]]; then
        printf 'UNKNOWN  %-60s %s%% — add an entry to scripts/coverage-floors.txt\n' "$pkg" "$pct"
        unknown=$((unknown + 1))
        continue
    fi
    # awk for float compare
    if awk -v p="$pct" -v f="$floor" 'BEGIN { exit !(p+0 < f+0) }'; then
        printf 'FAIL     %-60s %s%% < floor %s%%\n' "$pkg" "$pct" "$floor"
        violations=$((violations + 1))
    else
        printf 'ok       %-60s %s%% (floor %s%%)\n' "$pkg" "$pct" "$floor"
    fi
done <"$TEST_LOG"

echo
echo "Checked $checked packages; $violations regression(s), $unknown unknown package(s)."

if (( violations > 0 || unknown > 0 )); then
    exit 1
fi
