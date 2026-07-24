#!/usr/bin/env bash
# Windows Shell T3 e2e against the repo-standard stack (scripts/run-dev.sh).
#
# The Shell client never starts the server. This *harness* may start run-dev
# as a fixture when --start-run-dev is passed (test orchestration only).
#
# Usage:
#   # Terminal A: make run-dev
#   # Terminal B:
#   ./clients/windows-shell/scripts/e2e.sh
#
#   # Or one-shot (starts run-dev, runs tests, tears down):
#   ./clients/windows-shell/scripts/e2e.sh --start-run-dev
#
# Env:
#   AG_E2E_GATEWAY_URL   default http://127.0.0.1:8443
#   AG_E2E_RUN_DEV       set to 1 by this script
#   DOTNET / Windows:    when run from WSL, tests execute via powershell + win-test tree
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
cd "$ROOT"

# Default: attach to run-dev's usual :8443. --start-run-dev uses a dedicated
# port so a stale process on 8443 cannot fail the fixture bind.
GATEWAY_URL="${AG_E2E_GATEWAY_URL:-http://127.0.0.1:8443}"
START_RUN_DEV=0
SKIP_UNIT=0
SKIP_WINUI=0
SKIP_UI=0
FIXTURE_PORT=18443

usage() {
  cat <<'EOF'
Usage: e2e.sh [--start-run-dev] [--skip-unit] [--skip-winui] [--skip-ui] [--gateway-url URL]

  --start-run-dev   Start run-dev *backend* fixture (server -no-auth) on :18443
                    (or URL host:port if --gateway-url set)
  --skip-unit       Skip always-on unit tests; only run AG_E2E_RUN_DEV facts
  --skip-winui      Skip WinUI self-contained layout assert + launch smoke
                    (implies --skip-ui: the UI stage needs the smoke build)
  --skip-ui         Skip FlaUI UIA-driven panel tests (keep layout + smoke)
  --gateway-url     Override AG_E2E_GATEWAY_URL (default http://127.0.0.1:8443;
                    with --start-run-dev default becomes http://127.0.0.1:18443)

Stages:
  1) gateway probe (make run-dev or fixture)
  2) xUnit (Core/Platform + RunDev E2E facts when AG_E2E_RUN_DEV=1)
  3) WinUI layout + launch smoke (startup error log SoT; optional --skip-winui)
  4) WinUI UI automation via FlaUI/UIA (AG_E2E_WINUI_UI=1; optional --skip-ui)
EOF
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --start-run-dev) START_RUN_DEV=1; shift ;;
    --skip-unit) SKIP_UNIT=1; shift ;;
    --skip-winui) SKIP_WINUI=1; shift ;;
    --skip-ui) SKIP_UI=1; shift ;;
    --gateway-url) GATEWAY_URL="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

if [[ "$START_RUN_DEV" == "1" && -z "${AG_E2E_GATEWAY_URL:-}" ]]; then
  # Only rewrite default when caller did not pin AG_E2E_GATEWAY_URL / --gateway-url
  if [[ "$GATEWAY_URL" == "http://127.0.0.1:8443" ]]; then
    GATEWAY_URL="http://127.0.0.1:${FIXTURE_PORT}"
  fi
fi

export AG_E2E_RUN_DEV=1
export AG_E2E_GATEWAY_URL="$GATEWAY_URL"

probe() {
  curl -sf -o /dev/null "${GATEWAY_URL}/api/sessions" && return 0
  return 1
}

RUN_DEV_PID=""
cleanup() {
  if [[ -n "$RUN_DEV_PID" ]] && kill -0 "$RUN_DEV_PID" 2>/dev/null; then
    echo "Stopping run-dev (pid $RUN_DEV_PID)..."
    kill "$RUN_DEV_PID" 2>/dev/null || true
    wait "$RUN_DEV_PID" 2>/dev/null || true
  fi
}
trap cleanup EXIT INT TERM

if ! probe; then
  if [[ "$START_RUN_DEV" != "1" ]]; then
    echo "Gateway not reachable at $GATEWAY_URL" >&2
    echo "Start the fixture in WSL:" >&2
    echo "  make run-dev" >&2
    echo "Or re-run with --start-run-dev" >&2
    exit 1
  fi
  # Fixture = same backend posture as scripts/run-dev.sh (server -no-auth loopback).
  # We start only the gateway binary here so npm/web is not required for Shell e2e.
  # Operators may instead leave a full `make run-dev` running and omit --start-run-dev.
  echo "Starting run-dev *backend* fixture (server -no-auth; same as scripts/run-dev.sh)..."
  mkdir -p "$ROOT/.run-dev"
  : >"$ROOT/.run-dev/e2e-run-dev.log"
  export GOCACHE="${GOCACHE:-/tmp/gocache-agent-grid}"
  export GOMODCACHE="${GOMODCACHE:-/tmp/gomodcache-agent-grid}"
  mkdir -p "$GOCACHE" "$GOMODCACHE"
  (
    cd "$ROOT/src"
    go build -o "$ROOT/server" ./cmd/server
  ) >>"$ROOT/.run-dev/e2e-run-dev.log" 2>&1
  if [[ ! -x "$ROOT/server" ]]; then
    echo "go build server failed; log:" >&2
    cat "$ROOT/.run-dev/e2e-run-dev.log" >&2
    exit 1
  fi

  SERVER_DATA_DIR="$ROOT/.run-dev/server"
  mkdir -p "$SERVER_DATA_DIR"
  # Bind host:port from GATEWAY_URL (default 127.0.0.1:8443)
  BACKEND_ADDR="${GATEWAY_URL#http://}"
  BACKEND_ADDR="${BACKEND_ADDR#https://}"
  BACKEND_ADDR="${BACKEND_ADDR%%/*}"
  PORT_ONLY="${BACKEND_ADDR##*:}"

  # Free the fixture port if a stale process holds it (best-effort).
  if command -v fuser >/dev/null 2>&1; then
    fuser -k "${PORT_ONLY}/tcp" >/dev/null 2>&1 || true
    sleep 0.3
  fi

  # Logs go primarily to $SERVER_DATA_DIR/server.log (slog); keep stdout/err too.
  setsid "$ROOT/server" -insecure -no-auth -addr "$BACKEND_ADDR" -data-dir "$SERVER_DATA_DIR" \
    >>"$ROOT/.run-dev/e2e-run-dev.log" 2>&1 < /dev/null &
  RUN_DEV_PID=$!
  disown "$RUN_DEV_PID" 2>/dev/null || true

  for _ in $(seq 1 100); do
    if probe; then
      echo "run-dev backend ready at $GATEWAY_URL (pid $RUN_DEV_PID)"
      break
    fi
    if ! kill -0 "$RUN_DEV_PID" 2>/dev/null; then
      echo "server exited early; logs:" >&2
      tail -n 40 "$ROOT/.run-dev/e2e-run-dev.log" >&2 || true
      tail -n 40 "$SERVER_DATA_DIR/server.log" >&2 || true
      exit 1
    fi
    sleep 0.2
  done
  if ! probe; then
    echo "server did not become ready; logs:" >&2
    tail -n 40 "$ROOT/.run-dev/e2e-run-dev.log" >&2 || true
    tail -n 40 "$SERVER_DATA_DIR/server.log" >&2 || true
    exit 1
  fi
else
  echo "Using existing gateway at $GATEWAY_URL (e.g. make run-dev)"
fi

# Prefer Windows dotnet via robocopy tree when powershell is available (WSL host).
run_dotnet_tests() {
  local filter="${1:-}"
  if command -v powershell.exe >/dev/null 2>&1; then
    local repo_win
    repo_win=$(wslpath -w "$ROOT" 2>/dev/null || true)
    if [[ -z "$repo_win" ]]; then
      repo_win=$(powershell.exe -NoProfile -Command "(wsl.exe -e wslpath -w '$ROOT').Trim()" | tr -d '\r')
    fi
    powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "
      \$ErrorActionPreference = 'Stop'
      \$dotnet = Join-Path \$env:LOCALAPPDATA 'Microsoft\dotnet\dotnet.exe'
      if (-not (Test-Path \$dotnet)) { \$dotnet = 'C:\Program Files\dotnet\dotnet.exe' }
      \$env:DOTNET_ROOT = Split-Path \$dotnet
      \$env:PATH = \"\$env:DOTNET_ROOT;\$env:PATH\"
      \$env:AG_E2E_RUN_DEV = '1'
      \$env:AG_E2E_GATEWAY_URL = '$GATEWAY_URL'
      \$shell = Join-Path '$repo_win' 'clients\windows-shell'
      \$local = Join-Path \$env:LOCALAPPDATA 'Temp\ag-shell-src'
      if (Test-Path \$local) { Remove-Item -Recurse -Force \$local }
      & robocopy \$shell \$local /E /XD bin obj .git /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
      if (\$LASTEXITCODE -ge 8) { throw \"robocopy failed: \$LASTEXITCODE\" }
      Set-Location \$local
      if ('$filter' -ne '') {
        & \$dotnet test AgentGrid.Shell.sln --nologo --filter '$filter'
      } else {
        & \$dotnet test AgentGrid.Shell.sln --nologo
      }
      exit \$LASTEXITCODE
    "
  elif command -v dotnet >/dev/null 2>&1; then
    export DOTNET_CLI_HOME="${DOTNET_CLI_HOME:-/tmp/dotnet-home}"
    export NUGET_PACKAGES="${NUGET_PACKAGES:-/tmp/nuget-packages}"
    cd "$ROOT/clients/windows-shell"
    if [[ -n "$filter" ]]; then
      dotnet test AgentGrid.Shell.sln --nologo --filter "$filter"
    else
      dotnet test AgentGrid.Shell.sln --nologo
    fi
  else
    echo "neither powershell.exe nor dotnet available" >&2
    exit 2
  fi
}

echo "== Windows Shell e2e (run-dev fixture @ $GATEWAY_URL) =="
if [[ "$SKIP_UNIT" == "1" ]]; then
  run_dotnet_tests "FullyQualifiedName~RunDevGatewayE2E"
else
  # Full suite: unit always-on + e2e facts (enabled via AG_E2E_RUN_DEV=1)
  run_dotnet_tests ""
fi

run_winui_smoke() {
  if [[ "$SKIP_WINUI" == "1" ]]; then
    echo "Skipping WinUI layout/launch smoke (--skip-winui)"
    return 0
  fi
  if ! command -v powershell.exe >/dev/null 2>&1; then
    echo "WARN: powershell.exe not available — skip WinUI smoke (use Windows host)"
    return 0
  fi
  local repo_win
  repo_win=$(wslpath -w "$ROOT" 2>/dev/null || true)
  if [[ -z "$repo_win" ]]; then
    repo_win=$(powershell.exe -NoProfile -Command "(wsl.exe -e wslpath -w '$ROOT').Trim()" | tr -d '\r')
  fi
  echo "== WinUI e2e: self-contained build + layout + launch smoke =="
  powershell.exe -NoProfile -ExecutionPolicy Bypass -Command "
    \$ErrorActionPreference = 'Stop'
    \$repo = '$repo_win'
    \$scripts = Join-Path \$repo 'clients\windows-shell\scripts'
    # Build only (no nested unit re-run): inline minimal build then assert + smoke
    \$dotnet = Join-Path \$env:LOCALAPPDATA 'Microsoft\dotnet\dotnet.exe'
    if (-not (Test-Path \$dotnet)) { \$dotnet = 'C:\Program Files\dotnet\dotnet.exe' }
    \$env:DOTNET_ROOT = Split-Path \$dotnet
    \$env:PATH = \"\$env:DOTNET_ROOT;\$env:PATH\"
    \$env:DOTNET_CLI_TELEMETRY_OPTOUT = '1'
    \$shell = Join-Path \$repo 'clients\windows-shell'
    \$local = Join-Path \$env:LOCALAPPDATA 'Temp\ag-shell-src'
    Get-Process AgentGrid.Shell.WinUI -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
    Start-Sleep -Seconds 1
    if (Test-Path \$local) {
      try { Remove-Item -Recurse -Force \$local } catch { Start-Sleep 2; Remove-Item -Recurse -Force \$local }
    }
    & robocopy \$shell \$local /E /XD bin obj .git /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
    if (\$LASTEXITCODE -ge 8) { throw \"robocopy failed: \$LASTEXITCODE\" }
    Set-Location \$local
    & \$dotnet build AgentGrid.Shell.WinUI\AgentGrid.Shell.WinUI.csproj -c Debug -p:Platform=x64 -p:RuntimeIdentifier=win-x64 -p:WindowsAppSDKSelfContained=true -p:SelfContained=true --nologo
    if (\$LASTEXITCODE -ne 0) { exit \$LASTEXITCODE }
    \$exe = Join-Path \$local 'AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\win-x64\AgentGrid.Shell.WinUI.exe'
    if (-not (Test-Path \$exe)) { throw \"WinUI exe missing: \$exe\" }
    & (Join-Path \$scripts 'assert-winui-layout.ps1') -OutDir (Split-Path \$exe)
    if (\$LASTEXITCODE -ne 0) { exit \$LASTEXITCODE }
    \$env:AG_NO_AUTH = '1'
    \$env:AG_GATEWAY_URL = '$GATEWAY_URL'
    & (Join-Path \$scripts 'launch-smoke.ps1') -Exe \$exe -SmokeSeconds 5
    if (\$LASTEXITCODE -ne 0) { exit \$LASTEXITCODE }
    if ('$SKIP_UI' -eq '1') {
      Write-Host 'Skipping FlaUI UI automation stage (--skip-ui)'
      exit 0
    }
    Write-Host '== WinUI UI automation (FlaUI/UIA) =='
    \$env:AG_E2E_WINUI_UI = '1'
    \$env:AG_E2E_WINUI_EXE = \$exe
    \$env:AG_E2E_GATEWAY_URL = '$GATEWAY_URL'
    & \$dotnet test AgentGrid.Shell.WinUI.UiTests\AgentGrid.Shell.WinUI.UiTests.csproj --nologo --filter 'FullyQualifiedName~PanelWindowUiE2E'
    exit \$LASTEXITCODE
  "
}

run_winui_smoke

echo "PASS: windows-shell e2e against $GATEWAY_URL"
