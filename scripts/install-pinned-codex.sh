#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
role=${1:-}
case "$role" in
  schema|fidelity) ;;
  *)
    echo "usage: $0 <schema|fidelity>" >&2
    exit 2
    ;;
esac

version=$(node -e '
const registry = require(process.argv[1]);
const role = process.argv[2];
const version = registry.codex?.[role]?.version;
if (!/^\d+\.\d+\.\d+$/.test(version ?? "")) process.exit(2);
process.stdout.write(version);
' "$repo_root/test-harness/tool-versions.json" "$role")

echo "Installing Codex $role pin: $version" >&2
npm install --global "@openai/codex@$version"
