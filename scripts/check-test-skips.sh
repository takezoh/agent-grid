#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT/src"
go test ./internal/harnesspolicy -run '^TestRepositorySkipInventory$' -count=1
