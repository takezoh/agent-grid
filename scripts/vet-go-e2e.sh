#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
mapfile -t packages < <(cd "$repo_root/src" && go run ./cmd/harness-check --mode e2e-packages --root .. --registry test-harness/e2e-suites.json)
cd "$repo_root/src"
go vet -tags e2e "${packages[@]}"
