# Build & test AgentGrid.Shell on Windows (avoid UNC/WSL path ref issues).
# Usage (from WSL):
#   powershell.exe -NoProfile -ExecutionPolicy Bypass -File \\wsl.localhost\...\clients\windows-shell\scripts\win-test.ps1
# Or from Windows PowerShell after mapping the repo.

$ErrorActionPreference = "Stop"
$dotnet = Join-Path $env:LOCALAPPDATA "Microsoft\dotnet\dotnet.exe"
if (-not (Test-Path $dotnet)) {
  $dotnet = "C:\Program Files\dotnet\dotnet.exe"
}
if (-not (Test-Path $dotnet)) {
  throw "dotnet SDK not found. Install with: irm https://dot.net/v1/dotnet-install.ps1 | iex"
}

$env:DOTNET_ROOT = Split-Path $dotnet
$env:PATH = "$env:DOTNET_ROOT;$env:PATH"
$env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"

# Resolve repo: script lives at clients/windows-shell/scripts/
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$shellDir = Split-Path -Parent $scriptDir

# If we're on a UNC (\\wsl.localhost\...) path, robocopy to local temp first.
$local = Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src-$PID"
$copied = $false
if ($shellDir -match '^\\\\') {
  Write-Host "Syncing $shellDir -> $local"
  if (Test-Path $local) { Remove-Item -Recurse -Force $local }
  & robocopy $shellDir $local /E /XD bin obj .git /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
  if ($LASTEXITCODE -ge 8) { throw "robocopy failed: $LASTEXITCODE" }
  $work = $local
  $copied = $true
} else {
  $work = $shellDir
}

try {
  Set-Location $work
  Write-Host "dotnet test in $work"
  & $dotnet test AgentGrid.Shell.sln --nologo
  $testExit = $LASTEXITCODE
} finally {
  if ($copied) {
    Set-Location $env:TEMP
    Remove-Item -Recurse -Force $local -ErrorAction SilentlyContinue
  }
}
exit $testExit
