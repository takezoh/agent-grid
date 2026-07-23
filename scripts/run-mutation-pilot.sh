#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
MANIFEST="${MUTATION_MANIFEST:-$ROOT/test-harness/mutants.json}"
BASELINE="${MUTATION_BASELINE:-$ROOT/test-harness/mutation-baseline.json}"
ARTIFACT="${MUTATION_ARTIFACT:-$ROOT/mutation-results.json}"

node - "$ROOT" "$MANIFEST" "$BASELINE" "$ARTIFACT" "$0" <<'NODE'
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const crypto = require("node:crypto");
const {spawnSync} = require("node:child_process");
const [root, manifestPath, baselinePath, artifactPath, runnerPath] = process.argv.slice(2);
const seed = 20260711;
const operatorVersion = "v1";
const operators = new Set(["conditional-negation", "route-target-substitution", "event-drop", "codec-field-omission"]);
const hash = value => crypto.createHash("sha256").update(Buffer.isBuffer(value) ? Buffer.from(value.toString().replace(/\r\n/g, "\n")) : String(value).replace(/\r\n/g, "\n")).digest("hex");
const identity = mutant => `${mutant.path}@${mutant.start}:${mutant.end}#${mutant.operator}#${mutant.source_hash}`;
const commandText = mutant => `(cd ${mutant.working_dir} && ${mutant.command.map(JSON.stringify).join(" ")})`;
const started = Date.now();
let artifact = {schema_version:1, generated_at:new Date().toISOString(), seed, operator_set_version:operatorVersion, result:"fail", score:0, baseline_hash:"unreadable", runner_hash:hash(fs.readFileSync(runnerPath)), toolchain:{node:process.version}, mutants:[]};
fs.writeFileSync(artifactPath, `${JSON.stringify(artifact, null, 2)}\n`);

let manifest, baseline;
try {
  manifest = JSON.parse(fs.readFileSync(manifestPath, "utf8"));
  baseline = JSON.parse(fs.readFileSync(baselinePath, "utf8"));
} catch (error) {
  console.error(error.message);
  process.exit(1);
}
artifact.baseline_hash = hash(fs.readFileSync(baselinePath));
const errors = [];
if (manifest.version !== 1 || manifest.seed !== seed || manifest.operator_set_version !== operatorVersion) errors.push("manifest contract mismatch");
if (baseline.version !== 1 || baseline.seed !== seed || baseline.operator_set_version !== operatorVersion) errors.push("baseline contract mismatch");
if (baseline.runner_hash !== artifact.runner_hash) errors.push("runner hash drift");
const foundOperators = new Set(manifest.mutants.map(mutant => mutant.operator));
for (const operator of operators) if (!foundOperators.has(operator)) errors.push(`missing operator ${operator}`);
const baselineByID = new Map(baseline.mutants.map(mutant => [mutant.id, mutant]));

const archive = spawnSync("git", ["-C", root, "archive", "--format=tar", "HEAD"], {maxBuffer: 256 * 1024 * 1024});
if (archive.status !== 0) errors.push(`git archive failed: ${String(archive.stderr)}`);
for (const mutant of [...manifest.mutants].sort((a,b) => a.id.localeCompare(b.id))) {
  const outcome = {id:mutant.id, operator:mutant.operator, result:"survived", exit_code:0, duration_ms:0, reproduce:commandText(mutant)};
  artifact.mutants.push(outcome);
  if (Date.now() - started >= 15 * 60 * 1000) {
    outcome.result = "timeout";
    outcome.exit_code = 124;
    errors.push(`${mutant.id}: overall timeout`);
    continue;
  }
  const sourcePath = path.join(root, mutant.path);
  const source = fs.readFileSync(sourcePath);
  if (hash(source) !== mutant.source_hash || identity(mutant) !== mutant.id || mutant.end > source.length || source.subarray(mutant.start, mutant.end).toString() !== mutant.before) {
    outcome.result = "identity-drift";
    outcome.exit_code = 2;
    errors.push(`${mutant.id}: identity or span drift`);
    continue;
  }
  if (!operators.has(mutant.operator) || !baselineByID.has(mutant.id)) {
    outcome.result = "untrusted";
    outcome.exit_code = 2;
    errors.push(`${mutant.id}: absent from operator set or trusted baseline`);
    continue;
  }
  const temp = fs.mkdtempSync(path.join(os.tmpdir(), "agent-grid-mutant-"));
  try {
    const extract = spawnSync("tar", ["-xf", "-", "-C", temp], {input:archive.stdout, maxBuffer:256 * 1024 * 1024});
    if (extract.status !== 0) throw new Error(`archive extraction failed: ${String(extract.stderr)}`);
    const mutated = Buffer.concat([source.subarray(0, mutant.start), Buffer.from(mutant.replacement), source.subarray(mutant.end)]);
    fs.writeFileSync(path.join(temp, mutant.path), mutated);
    const sourceModules = path.join(root, "clients/ui/node_modules");
    const tempModules = path.join(temp, "clients/ui/node_modules");
    if (fs.existsSync(sourceModules) && !fs.existsSync(tempModules)) fs.symlinkSync(sourceModules, tempModules, "dir");
    const caseStarted = Date.now();
    const result = spawnSync(mutant.command[0], mutant.command.slice(1), {
      cwd:path.join(temp, mutant.working_dir), stdio:"inherit", timeout:Math.min(300, mutant.timeout_seconds) * 1000,
      env:{...process.env, MUTATION_SEED:String(seed), VITEST_POOL_ID:String(seed)},
    });
    outcome.duration_ms = Date.now() - caseStarted;
    if (result.error && result.error.code === "ETIMEDOUT") {
      outcome.result = "timeout"; outcome.exit_code = 124;
    } else if (result.error) {
      outcome.result = "runner-error"; outcome.exit_code = 125;
    } else {
      outcome.exit_code = result.status === null ? 130 : result.status;
      outcome.result = outcome.exit_code === 0 ? "survived" : "killed";
    }
  } finally {
    fs.rmSync(temp, {recursive:true, force:true});
  }
  if (outcome.result !== "killed" && baselineByID.get(mutant.id).must_kill) errors.push(`${mutant.id}: must-kill mutant ${outcome.result}`);
}

const killed = artifact.mutants.filter(mutant => mutant.result === "killed").length;
artifact.score = manifest.mutants.length === 0 ? 0 : killed / manifest.mutants.length;
if (artifact.score < baseline.minimum_score) errors.push(`score ${artifact.score} below baseline ${baseline.minimum_score}`);
const goVersion = spawnSync("go", ["version"], {encoding:"utf8"});
const npmVersion = spawnSync("npm", ["--version"], {encoding:"utf8"});
artifact.toolchain.go = goVersion.status === 0 ? goVersion.stdout.trim() : "unavailable";
artifact.toolchain.npm = npmVersion.status === 0 ? npmVersion.stdout.trim() : "unavailable";
artifact.duration_ms = Date.now() - started;
artifact.errors = errors.sort();
artifact.result = errors.length === 0 ? "pass" : "fail";
fs.writeFileSync(artifactPath, `${JSON.stringify(artifact, null, 2)}\n`);
if (errors.length > 0) {
  for (const error of errors) console.error(error);
  process.exit(1);
}
NODE
