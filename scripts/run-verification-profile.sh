#!/usr/bin/env bash
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
profile=${1:-}
artifact=${AG_HARNESS_PROFILE_ARTIFACT:-"$repo_root/test-harness/profile-result.json"}
if [[ -z "$profile" ]]; then
  echo "usage: $0 <save|pre-push|pr|nightly>" >&2
  exit 2
fi

mapfile -t commands < <(node - "$repo_root/test-harness/profiles.json" "$profile" <<'NODE'
const fs = require("node:fs");
const [path, name] = process.argv.slice(2);
const profile = JSON.parse(fs.readFileSync(path, "utf8")).profiles.find(value => value.name === name);
if (!profile) process.exit(2);
for (const command of profile.commands) console.log(command);
NODE
)
if (( ${#commands[@]} == 0 )); then
  echo "unknown or empty verification profile: $profile" >&2
  exit 2
fi

started=$(date +%s)
status=pass
for command in "${commands[@]}"; do
  if ! (cd "$repo_root" && bash -lc "$command"); then
    status=fail
    break
  fi
done
ended=$(date +%s)
mkdir -p "$(dirname "$artifact")"
printf '{"profile":"%s","status":"%s","duration_seconds":%d,"skipped":[]}\n' "$profile" "$status" "$((ended-started))" > "$artifact"
[[ "$status" == pass ]]
