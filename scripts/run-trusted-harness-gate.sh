#!/bin/sh
set -eu

repo_root=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
base_ref=${1:-origin/main}
request=${HARNESS_ESCALATION_REQUEST:-}
artifact=${HARNESS_TAMPERING_ARTIFACT:-$repo_root/harness-tampering-result.json}
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT HUP INT TERM

git -C "$repo_root" merge-base "$base_ref" HEAD > "$tmp/merge-base"
IFS= read -r merge_base < "$tmp/merge-base"
mkdir -p "$tmp/base"
git -C "$repo_root" archive --format=tar --output="$tmp/base.tar" "$merge_base"
tar -xf "$tmp/base.tar" -C "$tmp/base"

if [ ! -x "$tmp/base/scripts/check-harness-tampering.sh" ]; then
  cat > "$artifact" <<EOF
{"schema_version":1,"status":"bootstrap-required","merge_base":"$merge_base","reason":"trusted merge-base does not contain the harness checker; land the bootstrap through an explicitly reviewed direct merge, then require this gate for subsequent changes"}
EOF
  echo "trusted harness gate bootstrap required: merge-base has no checker" >&2
  exit 3
fi

set -- --base-root "$tmp/base" --head-root "$repo_root" --output "$artifact"
[ -z "$request" ] || set -- "$@" --request "$request"
"$tmp/base/scripts/check-harness-tampering.sh" "$@"
