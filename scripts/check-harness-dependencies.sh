#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
root=$repo_root
registry=test-harness/dependencies.json
boundaries=test-harness/external-boundaries.json
e2e_registry=test-harness/e2e-suites.json

case "${1:-}" in
  --fixture)
    [ "$#" -eq 2 ] || { echo "usage: $0 --fixture NAME" >&2; exit 2; }
    root=$repo_root/src/internal/harnesspolicy/testdata/dependencies/repo
    registry=../"$2".json
    boundaries=
    e2e_registry=e2e-suites.json
    ;;
  --registry)
    [ "$#" -eq 2 ] || { echo "usage: $0 --registry PATH" >&2; exit 2; }
    registry=$2
    ;;
  "") ;;
  *) echo "usage: $0 [--fixture NAME | --registry PATH]" >&2; exit 2 ;;
esac

cd "$repo_root/src"
go run ./cmd/harness-check --root "$root" --registry "$registry" --boundaries "$boundaries" --e2e-registry "$e2e_registry"
