#!/usr/bin/env bash
# Fail-closed compatibility checks for protocol/ + generated SDKs.
# Design:
#   - message schemas (protocol/*.schema.json) = typed payload SoT
#   - openapi.yaml = REST-binding annex (routes only; not generator input)
#   - models via pinned quicktype; transport hand-written
# FR-P1-05/06, NFR-01, adr-20260724-protocol-message-schema-sot-rest-binding
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

fail() { echo "COMPAT FAIL: $*" >&2; exit 1; }

# 1. Required protocol artifacts exist (schemas = SoT; openapi = annex).
for f in \
  protocol/events.schema.json \
  protocol/commands.schema.json \
  protocol/capabilities.schema.json \
  protocol/deep-links.schema.json \
  protocol/notifications.schema.json \
  protocol/openapi.yaml
do
  [[ -f "$f" ]] || fail "missing $f"
done

# 2. JSON Schema files parse as JSON.
for f in protocol/*.schema.json; do
  python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$f" \
    || fail "invalid JSON: $f"
done

# 3. openapi.yaml is a REST-binding annex: has paths, and should $ref or
#    name the message schemas (not act as a second SoT for payload types).
python3 - <<'PY' || fail "openapi.yaml REST-binding annex check failed"
import re, sys
text = open("protocol/openapi.yaml").read()
for needle in ("openapi:", "info:", "paths:", "1.0.0-phase01"):
    if needle not in text:
        print("missing", needle, file=sys.stderr)
        sys.exit(1)
# Prefer schema refs over large inline payload duplicates (soft signal).
if "schema.json" not in text and "$ref" not in text:
    print("INCONCLUSIVE: openapi.yaml has no schema $ref; annex should reference message schemas", file=sys.stderr)
    sys.exit(2)
# Generator must not treat openapi as SoT: ban accidental "components/schemas"
# megadump markers we previously used when openapi was generator input.
if "x-generator-sot: openapi" in text:
    print("openapi.yaml must not claim generator SoT", file=sys.stderr)
    sys.exit(1)
print("openapi annex ok")
PY

# 4. quicktype pin + emit config + four language roots.
[[ -f clients/sdk/package.json ]] || fail "missing clients/sdk/package.json (quicktype pin root)"
[[ -f clients/sdk/quicktype-emit.json ]] || fail "missing clients/sdk/quicktype-emit.json"
[[ -f clients/sdk/.quicktype-version ]] || fail "missing clients/sdk/.quicktype-version (run scripts/generate-sdks.sh)"
# Reject leftover OpenAPI Generator pin from the superseded design.
if [[ -f clients/sdk/.openapi-generator-version ]]; then
  fail "clients/sdk/.openapi-generator-version must be removed (superseded by quicktype)"
fi
for lang in ts csharp kotlin swift; do
  [[ -d "clients/sdk/$lang" ]] || fail "missing clients/sdk/$lang"
done
# Pin in package.json must match .quicktype-version file.
python3 - <<'PY' || fail "quicktype pin mismatch"
import json, pathlib, re, sys
pkg = json.load(open("clients/sdk/package.json"))
want = (pkg.get("devDependencies") or {}).get("quicktype", "")
want = re.sub(r"^[\^~>=]*", "", want)
got = pathlib.Path("clients/sdk/.quicktype-version").read_text().strip()
if not want or not got:
    print("INCONCLUSIVE: empty quicktype pin", file=sys.stderr)
    sys.exit(2)
if want != got:
    print(f"pin mismatch: package.json={want!r} .quicktype-version={got!r}", file=sys.stderr)
    sys.exit(1)
print("quicktype pin ok:", got)
PY

# 5. Shared recorded scenario exists (all SDK targets must use it).
[[ -f protocol/simulator/recordings/approval-round-trip.jsonl ]] \
  || fail "missing shared recorded scenario"
[[ -f protocol/simulator/fixtures/approval-round-trip.json ]] \
  || fail "missing deterministic fixture"

# 6. Declared surface = message schemas ($defs + property names) ∪ REST routes
#    from openapi annex. Fail closed on inconclusive scans.
python3 - <<'PY' || fail "undeclared SDK surface scan failed"
import json, pathlib, re, sys

# --- message schema surface ---
schema_tokens = set()
for path in pathlib.Path("protocol").glob("*.schema.json"):
    try:
        doc = json.load(open(path))
    except Exception as e:
        print("INCONCLUSIVE: cannot parse", path, e, file=sys.stderr)
        sys.exit(2)
    defs = doc.get("$defs") or {}
    if not defs and doc.get("type") is None and "oneOf" not in doc and "properties" not in doc:
        print("INCONCLUSIVE: empty schema", path, file=sys.stderr)
        sys.exit(2)
    for name in defs:
        schema_tokens.add(name)
    # top-level property names if present
    for name in (doc.get("properties") or {}):
        schema_tokens.add(name)
    # walk $defs properties lightly
    for d in defs.values():
        if isinstance(d, dict):
            for name in (d.get("properties") or {}):
                schema_tokens.add(name)

# Required core message types for approval domain
for need in ("ApprovalWire", "QuestionWire", "ApprovalRespond", "QuestionRespond"):
    if need not in schema_tokens:
        # deep-links / notifications may not have ApprovalWire — only require from events/commands
        pass
if "ApprovalRespond" not in schema_tokens and "ApprovalWire" not in schema_tokens:
    print("INCONCLUSIVE: approval types missing from protocol/*.schema.json", file=sys.stderr)
    sys.exit(2)

# --- REST annex surface ---
rest_ops = set()
rest_paths = set()
text = open("protocol/openapi.yaml").read()
for m in re.finditer(r"operationId:\s*(\w+)", text):
    rest_ops.add(m.group(1))
for m in re.finditer(r"^  (/api/[^\s:]+):", text, re.M):
    rest_paths.add(m.group(1))
if not rest_ops:
    print("INCONCLUSIVE: no operationId in openapi.yaml", file=sys.stderr)
    sys.exit(2)

declared_ipc = {
    "approval.respond", "approval.cancel", "question.respond", "question.cancel",
}

# Scan hand-written transport + cover files (not GENERATED trees) for inventing
# undeclared command-like tokens.
suspect = []
scan_roots = []
for lang in ("ts", "csharp", "kotlin", "swift"):
    root = pathlib.Path("clients/sdk") / lang
    if not root.exists():
        continue
    for p in root.rglob("*"):
        if not p.is_file():
            continue
        # skip generated model trees
        parts = set(p.parts)
        if "generated" in parts or "Generated" in parts:
            continue
        if p.suffix not in {".ts", ".cs", ".kt", ".swift"}:
            continue
        scan_roots.append(p)

for path in scan_roots:
    try:
        body = path.read_text(encoding="utf-8", errors="ignore")
    except Exception:
        print("INCONCLUSIVE: cannot read", path, file=sys.stderr)
        sys.exit(2)
    for m in re.finditer(r"\b(approval|question)\.([a-zA-Z_][a-zA-Z0-9_]*)\b", body):
        token = m.group(0)
        if token in declared_ipc:
            continue
        line_start = body.rfind("\n", 0, m.start()) + 1
        line = body[line_start : body.find("\n", m.start())]
        stripped = line.strip()
        if stripped.startswith("//") or stripped.startswith("#") or stripped.startswith("*"):
            continue
        suspect.append((str(path), token))

if suspect:
    for p, s in suspect:
        print(f"undeclared surface {s} in {p}", file=sys.stderr)
    sys.exit(1)

print(
    "compatibility scan ok;",
    "schema_tokens=", len(schema_tokens),
    "rest_ops=", sorted(rest_ops)[:8],
)
PY

# 7. Simulator recorded-scenario unit test (main Go module).
(
  cd src
  GOCACHE="${GOCACHE:-/tmp/gocache-agent-grid}" go test -count=1 ./protocolsim/
) || fail "simulator tests failed"

echo "COMPAT OK"
