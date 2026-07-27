#!/usr/bin/env bash
set -euo pipefail

WEB_ADDR="${AG_UI_E2E_SMOKE_ADDR:-127.0.0.1:4174}"
RUN_DIR="$(mktemp -d "${TMPDIR:-/tmp}/agent-grid-ui-smoke.XXXXXX")"

cleanup() {
  if [[ -n "${preview_pid:-}" ]]; then
    kill "$preview_pid" 2>/dev/null || true
    wait "$preview_pid" 2>/dev/null || true
  fi
  rm -rf "$RUN_DIR"
}
trap cleanup EXIT INT TERM

npm run preview -- --host "${WEB_ADDR%:*}" --port "${WEB_ADDR##*:}" --strictPort \
  >"$RUN_DIR/preview.log" 2>&1 &
preview_pid="$!"

for _ in $(seq 1 100); do
  if curl --fail --silent "http://$WEB_ADDR/" >/dev/null; then
    break
  fi
  if ! kill -0 "$preview_pid" 2>/dev/null; then
    break
  fi
  sleep 0.1
done
if ! curl --fail --silent "http://$WEB_ADDR/" >/dev/null; then
  tail -n 100 "$RUN_DIR/preview.log" >&2 || true
  exit 1
fi

npx playwright test "$@"
