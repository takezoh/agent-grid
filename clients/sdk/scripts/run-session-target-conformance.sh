#!/usr/bin/env bash
# Run SessionTarget discriminator conformance harnesses for shipped SDKs.
#
# Always attempts TypeScript. C# / Kotlin / Swift run only when their toolchain
# is present, unless AG_SDK_CONFORMANCE_REQUIRE_ALL=1 (CI), which fails closed
# on any missing toolchain or harness failure.
#
# Success is never exit-code-only: each real harness must emit a language-
# specific success sentinel. Zero-exit shims that print unrelated messages
# (e.g. "only dotnet test is supported") must not count as OK.
#
# Usage (from repo root or clients/sdk):
#   clients/sdk/scripts/run-session-target-conformance.sh
#   AG_SDK_CONFORMANCE_REQUIRE_ALL=1 clients/sdk/scripts/run-session-target-conformance.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SDK_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
REQUIRE_ALL="${AG_SDK_CONFORMANCE_REQUIRE_ALL:-0}"
CACHE_DIR="${AG_SDK_CONFORMANCE_CACHE:-${TMPDIR:-/tmp}/agent-grid-sdk-conformance}"
mkdir -p "$CACHE_DIR"

# Stable sentinels — emitted only by the real harness programs/tests.
readonly SENTINEL_TS="AG_SDK_SESSION_TARGET_CONFORMANCE_OK:ts"
readonly SENTINEL_CSHARP="AG_SDK_SESSION_TARGET_CONFORMANCE_OK:csharp"
readonly SENTINEL_KOTLIN="AG_SDK_SESSION_TARGET_CONFORMANCE_OK:kotlin"
readonly SENTINEL_SWIFT="AG_SDK_SESSION_TARGET_CONFORMANCE_OK:swift"

ran=0
skipped=0
failed=0

log() { printf '%s\n' "$*"; }
warn() { printf 'WARN: %s\n' "$*" >&2; }
fail_msg() { printf 'FAIL: %s\n' "$*" >&2; }

have() { command -v "$1" >/dev/null 2>&1; }

record_skip() {
  local lang="$1" reason="$2"
  skipped=$((skipped + 1))
  if [[ "$REQUIRE_ALL" == "1" ]]; then
    fail_msg "$lang harness required but skipped: $reason"
    failed=$((failed + 1))
  else
    warn "skip $lang harness: $reason"
  fi
}

# Capture harness stdout+stderr, re-emit for logs, require exit 0 and sentinel.
# Usage: run_with_sentinel <lang> <sentinel> <command> [args...]
run_with_sentinel() {
  local lang="$1"
  local sentinel="$2"
  shift 2
  local out_file rc
  out_file="$(mktemp "$CACHE_DIR/harness-out.XXXXXX")"
  set +e
  "$@" >"$out_file" 2>&1
  rc=$?
  set -e
  # Preserve full captured output in the runner log stream.
  cat "$out_file"
  if [[ $rc -ne 0 ]]; then
    fail_msg "$lang harness exited $rc"
    rm -f "$out_file"
    return 1
  fi
  if ! grep -qF -- "$sentinel" "$out_file"; then
    fail_msg "$lang harness missing success sentinel ($sentinel) despite exit 0"
    rm -f "$out_file"
    return 1
  fi
  rm -f "$out_file"
  ran=$((ran + 1))
  log "OK $lang"
  return 0
}

run_ts() {
  if ! have npm; then
    record_skip "ts" "npm not found"
    return 0
  fi
  log "== TypeScript SessionTarget conformance =="
  run_with_sentinel "ts" "$SENTINEL_TS" bash -c '
    set -euo pipefail
    cd "$1"
    if [[ ! -d node_modules ]]; then
      npm ci --ignore-scripts >/dev/null
    fi
    npm test
  ' bash "$SDK_ROOT"
}

run_csharp() {
  if ! have dotnet; then
    record_skip "csharp" "dotnet not found"
    return 0
  fi
  log "== C# SessionTarget conformance =="
  run_with_sentinel "csharp" "$SENTINEL_CSHARP" bash -c '
    set -euo pipefail
    cd "$1"
    export DOTNET_CLI_HOME="${DOTNET_CLI_HOME:-$2/dotnet-home}"
    export DOTNET_NOLOGO=1
    export DOTNET_SKIP_FIRST_TIME_EXPERIENCE=1
    export DOTNET_CLI_TELEMETRY_OPTOUT=1
    mkdir -p "$DOTNET_CLI_HOME"
    dotnet run --nologo -c Release --verbosity quiet
  ' bash "$SDK_ROOT/csharp/harness" "$CACHE_DIR"
}

# Pin kotlinx-serialization jars so generated @Serializable models compile.
# No alternative validation library — SessionTarget init / isValid is under test.
KOTLINX_SERIALIZATION_VERSION="1.7.3"

ensure_kotlin_jars() {
  local jar_dir="$CACHE_DIR/kotlin-jars"
  mkdir -p "$jar_dir"
  local core="$jar_dir/kotlinx-serialization-core-jvm-${KOTLINX_SERIALIZATION_VERSION}.jar"
  local json="$jar_dir/kotlinx-serialization-json-jvm-${KOTLINX_SERIALIZATION_VERSION}.jar"
  if [[ ! -f "$core" ]]; then
    curl -fsSL \
      "https://repo1.maven.org/maven2/org/jetbrains/kotlinx/kotlinx-serialization-core-jvm/${KOTLINX_SERIALIZATION_VERSION}/kotlinx-serialization-core-jvm-${KOTLINX_SERIALIZATION_VERSION}.jar" \
      -o "$core"
  fi
  if [[ ! -f "$json" ]]; then
    curl -fsSL \
      "https://repo1.maven.org/maven2/org/jetbrains/kotlinx/kotlinx-serialization-json-jvm/${KOTLINX_SERIALIZATION_VERSION}/kotlinx-serialization-json-jvm-${KOTLINX_SERIALIZATION_VERSION}.jar" \
      -o "$json"
  fi
  printf '%s' "$core:$json"
}

kotlin_stdlib_jar() {
  local kotlinc_bin
  kotlinc_bin="$(command -v kotlinc)"
  local lib_dir
  lib_dir="$(cd "$(dirname "$kotlinc_bin")/../lib" && pwd)"
  if [[ -f "$lib_dir/kotlin-stdlib.jar" ]]; then
    printf '%s' "$lib_dir/kotlin-stdlib.jar"
    return 0
  fi
  # Some installs ship versioned jars only.
  local candidate
  candidate="$(ls "$lib_dir"/kotlin-stdlib-*.jar 2>/dev/null | head -n1 || true)"
  if [[ -n "$candidate" ]]; then
    printf '%s' "$candidate"
    return 0
  fi
  return 1
}

run_kotlin() {
  if ! have kotlinc; then
    record_skip "kotlin" "kotlinc not found"
    return 0
  fi
  if ! have java; then
    record_skip "kotlin" "java not found"
    return 0
  fi
  if ! have curl; then
    record_skip "kotlin" "curl not found (needed to fetch kotlinx-serialization jars)"
    return 0
  fi
  local stdlib
  if ! stdlib="$(kotlin_stdlib_jar)"; then
    record_skip "kotlin" "kotlin-stdlib.jar not found next to kotlinc"
    return 0
  fi
  log "== Kotlin SessionTarget conformance =="
  local ser_cp
  ser_cp="$(ensure_kotlin_jars)"
  local cp="$ser_cp:$stdlib"
  local out_dir="$CACHE_DIR/kotlin-out"
  rm -rf "$out_dir"
  mkdir -p "$out_dir"
  run_with_sentinel "kotlin" "$SENTINEL_KOTLIN" bash -c '
    set -euo pipefail
    kotlinc \
      -cp "$1" \
      -d "$2" \
      "$3/kotlin/generated/repository.kt" \
      "$3/kotlin/harness/SessionTargetConformance.kt"
    java -cp "$2:$1" dev.agentgrid.client.harness.SessionTargetConformanceKt
  ' bash "$cp" "$out_dir" "$SDK_ROOT"
}

run_swift() {
  if ! have swiftc; then
    record_skip "swift" "swiftc not found"
    return 0
  fi
  log "== Swift SessionTarget conformance =="
  local out_bin="$CACHE_DIR/session-target-swift"
  run_with_sentinel "swift" "$SENTINEL_SWIFT" bash -c '
    set -euo pipefail
    swiftc \
      "$1/swift/Generated/repository.swift" \
      "$1/swift/harness/main.swift" \
      -o "$2"
    "$2"
  ' bash "$SDK_ROOT" "$out_bin"
}

log "SessionTarget cross-SDK conformance (require_all=$REQUIRE_ALL)"
log "sdk_root=$SDK_ROOT"

run_or_count_fail() {
  local name="$1"
  shift
  set +e
  "$@"
  local rc=$?
  set -e
  if [[ $rc -ne 0 ]]; then
    fail_msg "$name harness failed (exit $rc)"
    failed=$((failed + 1))
  fi
}

run_or_count_fail "TypeScript" run_ts
run_or_count_fail "C#" run_csharp
run_or_count_fail "Kotlin" run_kotlin
run_or_count_fail "Swift" run_swift

log "summary: ran=$ran skipped=$skipped failed=$failed"
if [[ "$failed" -ne 0 ]]; then
  exit 1
fi
if [[ "$ran" -eq 0 ]]; then
  fail_msg "no SessionTarget harness executed"
  exit 1
fi
exit 0
