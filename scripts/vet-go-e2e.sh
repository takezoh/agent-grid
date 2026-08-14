#!/usr/bin/env bash
# Vets the Go T3 fidelity suites under the e2e build tag, per module.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mapfile -t entries < <(
  cd "$repo_root/services"
  go run ./cmd/harness-check --mode e2e-packages --root .. --registry test-harness/e2e-suites.json
)

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

for module in "${module_order[@]}"; do
  mapfile -t packages <<< "${by_module[$module]}"
  (cd "$repo_root/$module" && GOWORK=off go vet -tags e2e "${packages[@]}")
done
