#!/usr/bin/env bash
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
json_output=""
if [[ ${1:-} == "--json-output" ]]; then
  json_output=${2:-}
  [[ -n "$json_output" ]] || { echo "--json-output requires a path" >&2; exit 2; }
  shift 2
fi
(( $# == 0 )) || { echo "usage: $0 [--json-output PATH]" >&2; exit 2; }

mapfile -t packages < <(
  cd "$root/src"
  go run ./cmd/harness-check --mode e2e-packages --root .. --registry test-harness/e2e-suites.json
)
(( ${#packages[@]} > 0 )) || { echo "T3 suite registry produced no packages" >&2; exit 2; }

if [[ -n "$json_output" ]]; then
  (cd "$root/src" && go test -json -tags e2e -count=1 "${packages[@]}") > "$json_output"
else
  cd "$root/src"
  go test -tags e2e -v -count=1 "${packages[@]}"
fi
