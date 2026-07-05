#!/usr/bin/env bash
set -euo pipefail

ROOT="${1:-src}"

if [[ ! -d "$ROOT" ]]; then
    echo "root not found: $ROOT" >&2
    exit 2
fi

status=0

while IFS= read -r dir; do
    has_sibling=0
    while IFS= read -r testfile; do
        if ! grep -Eq '^//go:build[[:space:]]+e2e$' "$testfile"; then
            has_sibling=1
            break
        fi
    done < <(find "$dir" -maxdepth 1 -type f -name '*_test.go' | sort)

    if (( has_sibling == 0 )); then
        echo "missing always-on sibling test in package: ${dir#$ROOT/}" >&2
        status=1
    fi
done < <(
    find "$ROOT" -type f -name '*_e2e_test.go' -print0 |
        xargs -0 -n1 dirname |
        sort -u
)

exit "$status"
