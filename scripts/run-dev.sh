#!/usr/bin/env bash
# Launch a fully-isolated dev stack: a fresh arc daemon (cmd/arc) under a
# scratch data dir, the backend gateway (cmd/server), and the web-client host
# (cmd/web). The scratch data dir means this never collides with the user's
# production arc daemon (~/.agent-reactor/) — they can run side by side.
#
# The daemon owns all pty sessions and exposes them over its Unix socket; the
# gateway translates browser REST/WS traffic into daemon proto calls; the web
# host serves the UI and reverse-proxies /api and /ws to the gateway. Ctrl-C
# stops every process this script started and removes the scratch data dir.
#
# Env overrides:
#   BACKEND_ADDR     gateway listen addr        (default 127.0.0.1:8443)
#   WEB_ADDR         web host listen addr       (default 127.0.0.1:8080)
#   TOKEN            bearer token               (default: openssl-generated)
#   ARC_DATA_DIR     scratch dir for daemon     (default $ROOT/.run-dev/arc)
#   KEEP_DATA_DIR    1 = preserve ARC_DATA_DIR on exit (default: remove)
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT"

BACKEND_ADDR="${BACKEND_ADDR:-127.0.0.1:8443}"
WEB_ADDR="${WEB_ADDR:-127.0.0.1:8080}"
TOKEN="${TOKEN:-$(openssl rand -hex 24 2>/dev/null || head -c 24 /dev/urandom | od -An -tx1 | tr -d ' \n')}"
ARC_DATA_DIR="${ARC_DATA_DIR:-$ROOT/.run-dev/arc}"
KEEP_DATA_DIR="${KEEP_DATA_DIR:-0}"
ARC_SOCKET="$ARC_DATA_DIR/arc.sock"
ARC_LOG="$ARC_DATA_DIR/arc.log"

# Safety net when invoked directly (make run-dev builds first).
[ -x ./arc ] || make build
[ -x ./server ] || make build-server
[ -x ./web ] || make build-web

mkdir -p "$ARC_DATA_DIR"

pids=()
cleanup() {
  kill "${pids[@]}" 2>/dev/null || true
  if [ "$KEEP_DATA_DIR" != "1" ]; then
    rm -rf "$ARC_DATA_DIR"
  fi
}
trap cleanup EXIT INT TERM

# Always launch a fresh daemon under ARC_DATA_DIR. Because the data dir is
# unique to this script invocation, there is no flock contention with the
# user's production arc daemon at ~/.agent-reactor/.
ROOST_DATA_DIR="$ARC_DATA_DIR" ./arc >"$ARC_LOG" 2>&1 &
pids+=("$!")

# Wait up to ~5s for the daemon to bind its socket before the gateway dials,
# otherwise the gateway floods stderr with backoff WARNs.
for _ in $(seq 1 50); do
  [ -S "$ARC_SOCKET" ] && break
  sleep 0.1
done
if [ ! -S "$ARC_SOCKET" ]; then
  echo "arc daemon did not create $ARC_SOCKET within 5s. Tail of $ARC_LOG:" >&2
  tail -n 30 "$ARC_LOG" >&2 || true
  exit 1
fi

./server -insecure -addr "$BACKEND_ADDR" -token "$TOKEN" -arc-sock "$ARC_SOCKET" &
pids+=("$!")
./web -insecure -addr "$WEB_ADDR" -server "http://$BACKEND_ADDR" &
pids+=("$!")

cat <<EOF

agent-reactor dev up (isolated from ~/.agent-reactor):
  data    : $ARC_DATA_DIR
  daemon  : $ARC_SOCKET
  backend : http://$BACKEND_ADDR
  web     : http://$WEB_ADDR

  Open →   http://$WEB_ADDR/#token=$TOKEN

  Daemon log: tail -f $ARC_LOG
  Set KEEP_DATA_DIR=1 to preserve $ARC_DATA_DIR across runs.

Ctrl-C to stop everything.
EOF

# Exit as soon as ANY process dies (e.g. the gateway fails to bind a port) so
# the EXIT trap tears the others down, instead of hanging behind the banner.
wait -n
