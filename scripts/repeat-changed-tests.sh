#!/usr/bin/env bash
set -uo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASE_REF="${BASE_REF:-origin/main}"
ARTIFACT="${REPEAT_ARTIFACT:-$ROOT/test-results-repeat.json}"
REPEAT=10
SEED=20260711

while (( $# > 0 )); do
    case "$1" in
        --base) BASE_REF="$2"; shift 2 ;;
        --artifact) ARTIFACT="$2"; shift 2 ;;
        *) echo "unknown argument: $1" >&2; exit 2 ;;
    esac
done

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

# Write the failure sentinel first. A signal, timeout, or shell abort therefore
# cannot leave behind a green or missing artifact.
node - "$ARTIFACT" <<'NODE'
const fs = require("node:fs");
const output = process.argv[2];
fs.writeFileSync(output, `${JSON.stringify({schema_version:1,generated_at:new Date().toISOString(),seed:20260711,repeat:10,result:"fail",suites:[{name:"repeat",required:true,result:"fail",duration_ms:0,cases:[{name:"incomplete",required:true,result:"interrupted",duration_ms:0,attempts:[{attempt:1,result:"interrupted",duration_ms:0,exit_code:130}]}]}]}, null, 2)}\n`);
NODE

if ! git -C "$ROOT" merge-base "$BASE_REF" HEAD > "$TMP/merge-base"; then
    : > "$TMP/name-status"
else
    MERGE_BASE="$(<"$TMP/merge-base")"
    if ! git -C "$ROOT" diff --name-status --find-renames "$MERGE_BASE...HEAD" > "$TMP/name-status"; then
        : > "$TMP/name-status"
    fi
fi

awk -F '\t' '{ for (i=2; i<=NF; i++) print $i }' "$TMP/name-status" | LC_ALL=C sort -u > "$TMP/paths"
: > "$TMP/go-targets"
: > "$TMP/ts-targets"

while IFS= read -r changed; do
    [[ -n "$changed" ]] || continue
    if [[ "$changed" == *.go ]]; then
        dir="${changed%/*}"
        [[ "$dir" == "$changed" ]] && dir="src"
        if find "$ROOT/$dir" -maxdepth 1 -type f -name '*_test.go' -print -quit 2>/dev/null | grep -q .; then
            package="./${dir#src/}"
            [[ "$dir" == "src" ]] && package="."
            printf '%s\n' "$package" >> "$TMP/go-targets"
        fi
    fi
    if [[ "$changed" =~ \.(test|spec)\.tsx?$ ]]; then
        dir="${changed%/*}"
        find "$ROOT/$dir" -maxdepth 1 -type f \( -name '*.test.ts' -o -name '*.test.tsx' -o -name '*.spec.ts' -o -name '*.spec.tsx' \) 2>/dev/null |
            sed "s#^$ROOT/##" >> "$TMP/ts-targets"
    elif [[ "$changed" =~ \.tsx?$ ]]; then
        dir="${changed%/*}"
        stem="$(basename "$changed")"
        stem="${stem%.*}"
        find "$ROOT/$dir" -maxdepth 1 -type f \( -name '*.test.ts' -o -name '*.test.tsx' -o -name '*.spec.ts' -o -name '*.spec.tsx' \) 2>/dev/null |
            sed "s#^$ROOT/##" >> "$TMP/ts-targets"
        rg -l --glob '*.{test,spec}.{ts,tsx}' "(\.\.?/)+${stem}(['\"]|$)" "$ROOT/src/client/web" 2>/dev/null |
            sed "s#^$ROOT/##" >> "$TMP/ts-targets" || true
    fi
done < "$TMP/paths"

LC_ALL=C sort -u "$TMP/go-targets" -o "$TMP/go-targets"
LC_ALL=C sort -u "$TMP/ts-targets" -o "$TMP/ts-targets"
[[ -s "$TMP/go-targets" ]] || printf './...\n' > "$TMP/go-targets"
[[ -s "$TMP/ts-targets" ]] || printf '__ALL__\n' > "$TMP/ts-targets"

: > "$TMP/attempts.ndjson"
overall=0
run_with_timeout() {
    node -e '
const {spawnSync} = require("node:child_process");
const result = spawnSync(process.argv[1], process.argv.slice(2), {stdio:"inherit", env:process.env, timeout:Number(process.env.REPEAT_TIMEOUT_SECONDS || 600) * 1000});
if (result.error && result.error.code === "ETIMEDOUT") process.exit(124);
if (result.error) { console.error(result.error.message); process.exit(125); }
process.exit(result.status === null ? 130 : result.status);
' "$@"
}

run_case() {
    local language="$1" target="$2" attempt start end duration exit_code result
    for attempt in $(seq 1 "$REPEAT"); do
        start="$(node -e 'process.stdout.write(String(Date.now()))')"
        if [[ "$language" == "go" ]]; then
            (cd "$ROOT/src" && GODEBUG="randautoseed=0" run_with_timeout go test -count=1 -shuffle="$SEED" "$target")
            exit_code=$?
        elif [[ "$target" == "__ALL__" ]]; then
            (cd "$ROOT/src/client/web" && VITEST_POOL_ID="$SEED" run_with_timeout npm test -- --run --sequence.seed "$SEED")
            exit_code=$?
        else
            relative="${target#src/client/web/}"
            (cd "$ROOT/src/client/web" && VITEST_POOL_ID="$SEED" run_with_timeout npm test -- --run "$relative" --sequence.seed "$SEED")
            exit_code=$?
        fi
        end="$(node -e 'process.stdout.write(String(Date.now()))')"
        duration=$((end - start))
        result="pass"
        if (( exit_code == 124 )); then
            result="timeout"
            overall=1
        elif (( exit_code != 0 )); then
            result="fail"
            overall=1
        fi
        node -e 'console.log(JSON.stringify({suite:process.argv[1],case:process.argv[2],attempt:Number(process.argv[3]),duration_ms:Number(process.argv[4]),result:process.argv[5],exit_code:Number(process.argv[6])}))' \
            "$language" "$target" "$attempt" "$duration" "$result" "$exit_code" >> "$TMP/attempts.ndjson"
    done
}

while IFS= read -r target; do run_case go "$target"; done < "$TMP/go-targets"
while IFS= read -r target; do run_case typescript "$target"; done < "$TMP/ts-targets"

node - "$ARTIFACT" "$TMP/attempts.ndjson" "$overall" <<'NODE'
const fs = require("node:fs");
const [output, input, status] = process.argv.slice(2);
const rows = fs.readFileSync(input, "utf8").trim().split("\n").filter(Boolean).map(JSON.parse);
const grouped = new Map();
for (const row of rows) {
  const key = `${row.suite}\0${row.case}`;
  if (!grouped.has(key)) grouped.set(key, {name: row.case, required: true, result: "pass", duration_ms: 0, attempts: []});
  const item = grouped.get(key);
  item.attempts.push({attempt: row.attempt, result: row.result, duration_ms: row.duration_ms, exit_code: row.exit_code});
  item.duration_ms += row.duration_ms;
  if (row.result !== "pass") item.result = "fail";
}
const suites = ["go", "typescript"].map(name => {
  const cases = [...grouped.entries()].filter(([key]) => key.startsWith(`${name}\0`)).map(([, value]) => value).sort((a,b) => a.name.localeCompare(b.name));
  return {name, required: true, result: cases.length > 0 && cases.every(value => value.result === "pass" && value.attempts.length === 10) ? "pass" : "fail", duration_ms: cases.reduce((sum,value) => sum + value.duration_ms, 0), cases};
});
const failed = status !== "0" || suites.some(suite => suite.result !== "pass");
fs.writeFileSync(output, `${JSON.stringify({schema_version:1,generated_at:new Date().toISOString(),seed:20260711,repeat:10,result:failed?"fail":"pass",suites}, null, 2)}\n`);
if (failed) process.exitCode = 1;
NODE
artifact_status=$?
if (( overall != 0 || artifact_status != 0 )); then exit 1; fi
