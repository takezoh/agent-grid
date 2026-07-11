#!/usr/bin/env bash
set -euo pipefail

if (( $# < 2 )); then
    echo "usage: $0 OUTPUT INPUT.json [INPUT.json ...]" >&2
    exit 2
fi

OUTPUT="$1"
shift

node - "$OUTPUT" "$@" <<'NODE'
const fs = require("node:fs");
const [output, ...inputs] = process.argv.slice(2);
const suites = [];
let failed = false;

for (const input of inputs.sort()) {
  let value;
  try {
    value = JSON.parse(fs.readFileSync(input, "utf8"));
  } catch (error) {
    console.error(`cannot read result ${input}: ${error.message}`);
    failed = true;
    continue;
  }
  const incoming = Array.isArray(value.suites) ? value.suites : [value];
  for (const suite of incoming) {
    if (!suite || typeof suite.name !== "string" || !Array.isArray(suite.cases) || suite.cases.length === 0) {
      console.error(`result ${input} has no case-level suite data`);
      failed = true;
      continue;
    }
    suite.cases.sort((a, b) => String(a.name).localeCompare(String(b.name)));
    for (const testCase of suite.cases) {
      if (!Array.isArray(testCase.attempts) || testCase.attempts.length === 0) failed = true;
      if (testCase.required && testCase.result !== "pass") failed = true;
      if (testCase.result === "skip" && !testCase.skip_reason) failed = true;
      if (["fail", "timeout", "interrupted"].includes(testCase.result)) failed = true;
    }
    if (suite.required && suite.result !== "pass") failed = true;
    if (suite.result === "skip" && !suite.skip_reason) failed = true;
    if (suite.result !== "pass") failed = true;
    suites.push(suite);
  }
}

if (suites.length === 0) failed = true;
suites.sort((a, b) => a.name.localeCompare(b.name));
const artifact = {
  schema_version: 1,
  generated_at: new Date().toISOString(),
  seed: 20260711,
  repeat: 10,
  result: failed ? "fail" : "pass",
  suites,
};
fs.writeFileSync(output, `${JSON.stringify(artifact, null, 2)}\n`);
if (failed) process.exitCode = 1;
NODE
