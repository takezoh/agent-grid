#!/usr/bin/env bash
# Runs the Go T3 fidelity suites registered in test-harness/e2e-suites.json.
#
# Suites are grouped by the module they belong to, because the repository has
# more than one Go module and an independent client module cannot be reached
# with a package pattern from the services module. A suite that names no module
# belongs to services, which is where every suite lived when this ran a single
# `go test`.
set -euo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
json_output=""
if [[ ${1:-} == "--json-output" ]]; then
  json_output=${2:-}
  [[ -n "$json_output" ]] || { echo "--json-output requires a path" >&2; exit 2; }
  shift 2
fi
(( $# == 0 )) || { echo "usage: $0 [--json-output PATH]" >&2; exit 2; }

mapfile -t entries < <(
  cd "$root/services"
  go run ./cmd/harness-check --mode e2e-packages --root .. --registry test-harness/e2e-suites.json
)
(( ${#entries[@]} > 0 )) || { echo "T3 suite registry produced no packages" >&2; exit 2; }

declare -A by_module=()
declare -a module_order=()
for entry in "${entries[@]}"; do
  module=${entry%%$'\t'*}
  package=${entry#*$'\t'}
  if [[ -z ${by_module[$module]+set} ]]; then
    module_order+=("$module")
    by_module[$module]=$package
  else
    by_module[$module]+=$'\n'$package
  fi
done

if [[ -n "$json_output" ]]; then
  : > "$json_output"
fi

for module in "${module_order[@]}"; do
  mapfile -t packages <<< "${by_module[$module]}"
  if [[ -n "$json_output" ]]; then
    (cd "$root/$module" && GOWORK=off go test -json -tags e2e -count=1 "${packages[@]}") \
      >> "$json_output"
  else
    (cd "$root/$module" && GOWORK=off go test -tags e2e -v -count=1 "${packages[@]}")
  fi
done
