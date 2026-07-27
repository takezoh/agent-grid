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
CONFIG_DIR="$RUN_DIR/client-config"
SERVER_HOME="$RUN_DIR/home"
mkdir -p "$FAKE_BIN_DIR" "$DATA_DIR" "$CONFIG_DIR" "$SERVER_HOME/.agent-grid"

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
  cd "$ROOT/services"
  go build -o "$FAKE_BIN_DIR/fake-agents" ./server/api/testsupport/fakeagents
)
cp "$FAKE_BIN_DIR/fake-agents" "$FAKE_BIN_DIR/codex"

cat >"$SERVER_HOME/.agent-grid/settings.toml" <<EOF
[projects]
project_roots = ["$ROOT"]
project_paths = ["$ROOT"]

[[session.command_profiles]]
id = "codex"
label = "codex"
executable = "codex"
EOF

HOME="$SERVER_HOME" PATH="$FAKE_BIN_DIR:$PATH" AG_UI_E2E_STATUS_SCENARIOS=1 \
  "$ROOT/dist/bin/server" -insecure -no-auth -addr "$BACKEND_ADDR" -data-dir "$DATA_DIR" \
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

printf '%s\n' \
  '{"schema_version":2,"servers":[{"display_name":"E2E","enabled":true,"url":"http://'"$BACKEND_ADDR"'","auth":{"mode":"none"}}]}' \
  >"$CONFIG_DIR/servers.json"
"$ROOT/dist/bin/web-host" -insecure -addr "$WEB_ADDR" -config-dir "$CONFIG_DIR" \
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
npx playwright test --config playwright.real.config.ts "$@"
