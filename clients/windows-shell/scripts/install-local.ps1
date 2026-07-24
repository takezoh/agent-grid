# Publish WinUI (preferred) + optional headless Host into %LOCALAPPDATA%\agent-grid.
param(
  [string]$Configuration = "Release",
  [switch]$HostOnly
)

$ErrorActionPreference = "Stop"
$dotnet = Join-Path $env:LOCALAPPDATA "Microsoft\dotnet\dotnet.exe"
if (-not (Test-Path $dotnet)) { $dotnet = "C:\Program Files\dotnet\dotnet.exe" }
$env:DOTNET_ROOT = Split-Path $dotnet
$env:PATH = "$env:DOTNET_ROOT;$env:PATH"
$env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"

$repo = (wsl.exe -e wslpath -w /workspace/agent-grid).Trim()
$shell = Join-Path $repo "clients\windows-shell"
$localSrc = Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src"
if (Test-Path $localSrc) { Remove-Item -Recurse -Force $localSrc }
& robocopy $shell $localSrc /E /XD bin obj .git /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null

$dest = Join-Path $env:LOCALAPPDATA "agent-grid"
New-Item -ItemType Directory -Force -Path $dest | Out-Null
Set-Location $localSrc

if (-not $HostOnly) {
  $publish = Join-Path $dest "shell-winui"
  if (Test-Path $publish) { Remove-Item -Recurse -Force $publish }
  & $dotnet publish AgentGrid.Shell.WinUI\AgentGrid.Shell.WinUI.csproj `
    -c $Configuration -p:Platform=x64 -o $publish --nologo
  if ($LASTEXITCODE -ne 0) { throw "WinUI publish failed" }
  $exe = Join-Path $publish "AgentGrid.Shell.WinUI.exe"
  Write-Host "Published WinUI to $publish"
  & (Join-Path $shell "scripts\register-deep-link.ps1") -HostExe $exe
  Write-Host "Run: & '$exe'"
}

$hostOut = Join-Path $dest "shell-host"
if (Test-Path $hostOut) { Remove-Item -Recurse -Force $hostOut }
& $dotnet publish AgentGrid.Shell.Host\AgentGrid.Shell.Host.csproj -c $Configuration -o $hostOut --nologo
Write-Host "Published Host to $hostOut"
