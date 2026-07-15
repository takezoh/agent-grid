#!/usr/bin/env bash
set -uo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
profile=${1:-}
group=${2:-}
artifact=${AG_HARNESS_PROFILE_ARTIFACT:-"$repo_root/test-harness/profile-result.json"}
if [[ -z "$profile" ]]; then
  echo "usage: $0 <save|pre-push|pr|nightly> [group]" >&2
  exit 2
fi

records=$(mktemp)
trap 'rm -f "$records"' EXIT
mapfile -t commands < <(node - "$repo_root/test-harness/profiles.json" "$profile" "$group" <<'NODE'
const fs = require("node:fs");
const [path, name, group] = process.argv.slice(2);
const profile = JSON.parse(fs.readFileSync(path, "utf8")).profiles.find(value => value.name === name);
if (!profile) process.exit(2);
for (const item of profile.commands) {
  if (group && item.group !== group) continue;
  console.log([item.id, item.group, item.when || "", Buffer.from(item.command).toString("base64")].join("\t"));
}
NODE
)
if (( ${#commands[@]} == 0 )); then
  echo "unknown or empty verification profile/group: $profile ${group:-all}" >&2
  exit 2
fi

profile_started=$(date +%s)
profile_status=pass
for encoded in "${commands[@]}"; do
  IFS=$'\t' read -r id command_group when command_b64 <<<"$encoded"
  command=$(printf '%s' "$command_b64" | base64 --decode)
  if [[ "$when" == "pull-request" && -z "${BASE_REF:-}" ]]; then
    node -e 'console.log(JSON.stringify({id:process.argv[1],group:process.argv[2],status:"skip",duration_seconds:0,reason:"BASE_REF is not set"}))' "$id" "$command_group" >> "$records"
    continue
  fi
  echo "verification[$profile/$command_group]: $id" >&2
  started=$(date +%s)
  (cd "$repo_root" && bash -lc "$command")
  exit_code=$?
  ended=$(date +%s)
  if (( exit_code == 0 )); then command_status=pass; else command_status=fail; profile_status=fail; fi
  node -e 'console.log(JSON.stringify({id:process.argv[1],group:process.argv[2],status:process.argv[3],duration_seconds:Number(process.argv[4]),exit_code:Number(process.argv[5])}))' "$id" "$command_group" "$command_status" "$((ended-started))" "$exit_code" >> "$records"
  if (( exit_code != 0 )); then break; fi
done
profile_ended=$(date +%s)
mkdir -p "$(dirname "$artifact")"
node - "$records" "$artifact" "$profile" "${group:-all}" "$profile_status" "$((profile_ended-profile_started))" <<'NODE'
const fs = require("node:fs");
const [records, artifact, profile, group, status, duration] = process.argv.slice(2);
const commands = fs.readFileSync(records, "utf8").trim().split("\n").filter(Boolean).map(JSON.parse);
fs.writeFileSync(artifact, JSON.stringify({version:1,profile,group,status,duration_seconds:Number(duration),commands}, null, 2) + "\n");
NODE
[[ "$profile_status" == pass ]]
