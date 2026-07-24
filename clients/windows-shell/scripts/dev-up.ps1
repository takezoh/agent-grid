# Client-only launcher for Windows Shell (WinUI).
# Does NOT start the gateway.
#
# e2e stack (recommended):
#   Terminal A (WSL):  make run-dev          # scripts/run-dev.sh — server+web, -no-auth loopback
#   Terminal B (Win):  this script          # build + launch Shell; AG_NO_AUTH=1 by default
#
# Product/auth path: pass -NoAuth:$false and -TokenPath to a real gateway-token file.
param(
  [string]$GatewayUrl = "http://127.0.0.1:8443",
  [string]$TokenPath = "",
  [bool]$NoAuth = $true,
  [switch]$NoLaunch,
  [switch]$SkipBuild,
  [switch]$SkipRegister
)

$ErrorActionPreference = "Stop"

Write-Host "== agent-grid WinUI client =="
Write-Host "Gateway: $GatewayUrl (connect only)"
Write-Host "NoAuth:  $NoAuth  (match scripts/run-dev.sh -no-auth for e2e)"

if (-not $TokenPath) {
  $TokenPath = Join-Path $env:LOCALAPPDATA "agent-grid\gateway-token"
}

$repo = (wsl.exe -e wslpath -w /workspace/agent-grid).Trim()

if (-not $SkipBuild) {
  & (Join-Path $repo "clients\windows-shell\scripts\win-build-winui.ps1")
  if ($LASTEXITCODE -ne 0) { throw "win-build-winui failed" }
}

$exe = Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\AgentGrid.Shell.WinUI.exe"
if (-not (Test-Path $exe)) { throw "WinUI exe missing: $exe — run without -SkipBuild" }

if (-not $SkipRegister) {
  & (Join-Path $repo "clients\windows-shell\scripts\register-deep-link.ps1") -HostExe $exe
}

# Soft probe — server must already be up via make run-dev (or product daemon).
try {
  $headers = @{}
  if (-not $NoAuth) {
    if (-not (Test-Path $TokenPath)) { throw "token file missing: $TokenPath" }
    $tok = (Get-Content $TokenPath -Raw).Trim()
    $headers["Authorization"] = "Bearer $tok"
  }
  $r = Invoke-WebRequest -Uri ($GatewayUrl.TrimEnd('/') + "/api/sessions") `
    -Headers $headers -UseBasicParsing -TimeoutSec 3
  Write-Host "Probe $GatewayUrl/api/sessions -> $($r.StatusCode)"
} catch {
  Write-Host "Probe failed — is 'make run-dev' running in WSL?"
  Write-Host "  $($_.Exception.Message)"
}

if ($NoLaunch) {
  Write-Host "Ready: $exe"
  Write-Host "  AG_GATEWAY_URL=$GatewayUrl"
  Write-Host "  AG_NO_AUTH=$(if ($NoAuth) { '1' } else { '0' })"
  if (-not $NoAuth) { Write-Host "  AG_TOKEN_PATH=$TokenPath" }
  exit 0
}

Write-Host "Starting WinUI..."
$env:AG_GATEWAY_URL = $GatewayUrl
$env:AG_NO_AUTH = if ($NoAuth) { "1" } else { "0" }
$env:AG_TOKEN_PATH = $TokenPath
Start-Process -FilePath $exe
Start-Sleep -Seconds 2
$proc = Get-Process -Name "AgentGrid.Shell.WinUI" -ErrorAction SilentlyContinue
if ($proc) {
  Write-Host "WinUI running (pid=$($proc.Id))"
} else {
  Write-Host "WARN: process not listed — check tray / crash dialog"
}
