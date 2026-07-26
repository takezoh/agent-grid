#!/usr/bin/env bash
# Deterministic UI/UX E2E stack: real server + real uihost + fake Codex.
# This is intentionally separate from the default browser smoke, whose
# in-page fake backend keeps the PR gate fast.
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
BACKEND_ADDR="${AG_UI_E2E_BACKEND_ADDR:-127.0.0.1:18444}"
WEB_ADDR="${AG_UI_E2E_WEB_ADDR:-127.0.0.1:18081}"
RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agent-grid-ui-e2e.XXXXXX")"
FAKE_BIN_DIR="$RUN_DIR/bin"
DATA_DIR="$RUN_DIR/server"
mkdir -p "$FAKE_BIN_DIR" "$DATA_DIR"

pids=()
cleanup() {
  for pid in "${pids[@]:-}"; do
    kill "$pid" 2>/dev/null || true
  done
  for _ in $(seq 1 50); do
    alive=0
    for pid in "${pids[@]:-}"; do
      if kill -0 "$pid" 2>/dev/null; then
        alive=1
      fi
    done
    if [[ "$alive" == "0" ]]; then
      break
    fi
    sleep 0.1
  done
  for pid in "${pids[@]:-}"; do
    if kill -0 "$pid" 2>/dev/null; then
      kill -KILL "$pid" 2>/dev/null || true
    fi
  done
  wait "${pids[@]:-}" 2>/dev/null || true
  rm -rf "$RUN_DIR"
}
trap cleanup EXIT INT TERM

make -C "$ROOT" build-server build-web
(
  cd "$ROOT/src"
  go build -o "$FAKE_BIN_DIR/fake-agents" ./server/api/testsupport/fakeagents
)
cp "$FAKE_BIN_DIR/fake-agents" "$FAKE_BIN_DIR/codex"

PATH="$FAKE_BIN_DIR:$PATH" AG_UI_E2E_STATUS_SCENARIOS=1 \
  "$ROOT/server" -insecure -no-auth -addr "$BACKEND_ADDR" -data-dir "$DATA_DIR" \
  >"$RUN_DIR/server.log" 2>&1 &
pids+=("$!")

for _ in $(seq 1 100); do
  if curl --fail --silent "http://$BACKEND_ADDR/api/sessions" >/dev/null; then
    break
  fi
  sleep 0.1
done
if ! curl --fail --silent "http://$BACKEND_ADDR/api/sessions" >/dev/null; then
  tail -n 100 "$RUN_DIR/server.log" >&2 || true
  exit 1
fi

"$ROOT/web" -insecure -addr "$WEB_ADDR" -server "http://$BACKEND_ADDR" \
  >"$RUN_DIR/uihost.log" 2>&1 &
pids+=("$!")

for _ in $(seq 1 100); do
  if curl --fail --silent "http://$WEB_ADDR/" >/dev/null; then
    break
  fi
  sleep 0.1
done
if ! curl --fail --silent "http://$WEB_ADDR/" >/dev/null; then
  tail -n 100 "$RUN_DIR/uihost.log" >&2 || true
  exit 1
fi

echo "UI/UX E2E stack ready: web=http://$WEB_ADDR backend=http://$BACKEND_ADDR"
wait -n
