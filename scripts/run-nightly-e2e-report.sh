#!/usr/bin/env bash
set -uo pipefail

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output=${NIGHTLY_E2E_ARTIFACT:-"$root/nightly-e2e-results.json"}
raw=$(mktemp)
trap 'rm -f "$raw"' EXIT
printf '{"schema_version":1,"result":"fail","suites":[]}\n' > "$output"

status=0
"$root/scripts/run-go-e2e.sh" --json-output "$raw" || status=$?

node - "$raw" "$output" "$status" <<'NODE'
const fs = require("node:fs");
const [input, output, exitText] = process.argv.slice(2);
const events = fs.readFileSync(input, "utf8").split("\n").filter(Boolean).map(line => JSON.parse(line));
const cases = new Map();
for (const event of events) {
  if (!event.Test || !["pass", "fail", "skip"].includes(event.Action)) continue;
  cases.set(`${event.Package}\0${event.Test}`, {name:event.Test,required:true,result:event.Action,duration_ms:Math.round((event.Elapsed||0)*1000),attempts:[{attempt:1,result:event.Action,duration_ms:Math.round((event.Elapsed||0)*1000),exit_code:event.Action==="pass"?0:1}],...(event.Action==="skip"?{skip_reason:"go test reported a case-level skip"}:{})});
}
const byPackage = new Map();
for (const [key, testCase] of cases) { const pkg=key.split("\0",1)[0]; if(!byPackage.has(pkg))byPackage.set(pkg,[]); byPackage.get(pkg).push(testCase); }
const suites=[...byPackage].sort(([a],[b])=>a.localeCompare(b)).map(([name,items])=>{items.sort((a,b)=>a.name.localeCompare(b.name));const pass=items.length>0&&items.every(v=>v.result==="pass");return{name,required:true,result:pass?"pass":"fail",duration_ms:items.reduce((s,v)=>s+v.duration_ms,0),cases:items};});
const failed=Number(exitText)!==0||suites.length===0||suites.some(s=>s.result!=="pass");
fs.writeFileSync(output,`${JSON.stringify({schema_version:1,generated_at:new Date().toISOString(),seed:20260711,repeat:1,result:failed?"fail":"pass",suites},null,2)}\n`);
if(failed)process.exitCode=1;
NODE
