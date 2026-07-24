#!/usr/bin/env bash
# T3: setsid+nohup detach survival (run inside WSL).
set -euo pipefail

SERVER_PATH="${SERVER_PATH:-/workspace/agent-grid/server}"
PORT="${PORT:-18443}"
TOKEN_PATH="${TOKEN_PATH:-/tmp/ag-spike-token}"
DATA_DIR="${DATA_DIR:-/tmp/ag-spike-data}"
WAIT_SECONDS="${WAIT_SECONDS:-6}"
TOKEN_VALUE="spike-token-fixed"

echo "== WSL detach spike (in-distro) =="
echo "server=$SERVER_PATH port=$PORT dataDir=$DATA_DIR"

if [[ ! -x "$SERVER_PATH" ]]; then
  echo "server not executable at $SERVER_PATH" >&2
  exit 1
fi

rm -rf "$DATA_DIR"
mkdir -p "$DATA_DIR"
printf '%s' "$TOKEN_VALUE" > "$TOKEN_PATH"
chmod 600 "$TOKEN_PATH"
fuser -k "${PORT}/tcp" 2>/dev/null || true
sleep 0.3

# Match WslDaemonRunner.BuildDetachCommand
setsid nohup "$SERVER_PATH" \
  -data-dir "$DATA_DIR" \
  -addr "127.0.0.1:${PORT}" \
  -token-file "$TOKEN_PATH" \
  -insecure \
  >/tmp/agent-grid-spike.log 2>&1 </dev/null &
PID=$!
echo "spawned pid=$PID"
sleep 2

if [[ ! -d "/proc/$PID" ]]; then
  echo "daemon died immediately:" >&2
  tail -n 40 /tmp/agent-grid-spike.log >&2 || true
  exit 1
fi

echo "PPid: $(awk '/^PPid:/{print $2}' "/proc/$PID/status")"

probe() {
  curl -s -o /dev/null -w "%{http_code}" \
    -H "Authorization: Bearer ${TOKEN_VALUE}" \
    "http://127.0.0.1:${PORT}/api/sessions" || echo 000
}

CODE1=$(probe)
echo "probe1 status=$CODE1"
if [[ "$CODE1" != "200" ]]; then
  echo "initial probe failed" >&2
  tail -n 40 /tmp/agent-grid-spike.log >&2 || true
  kill "$PID" 2>/dev/null || true
  exit 1
fi

echo "waiting ${WAIT_SECONDS}s..."
sleep "$WAIT_SECONDS"

CODE2=$(probe)
ALIVE=no
[[ -d "/proc/$PID" ]] && ALIVE=yes
echo "probe2 status=$CODE2 alive=$ALIVE"
echo "PPid after wait: $(awk '/^PPid:/{print $2}' "/proc/$PID/status" 2>/dev/null || true)"

kill "$PID" 2>/dev/null || true
wait "$PID" 2>/dev/null || true

if [[ "$CODE2" != "200" || "$ALIVE" != "yes" ]]; then
  echo "FAIL: did not survive detach window" >&2
  exit 1
fi

echo "PASS: setsid+nohup detach survived ${WAIT_SECONDS}s with /api/sessions OK"
