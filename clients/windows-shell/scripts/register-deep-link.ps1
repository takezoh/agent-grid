# Register agent-grid:// protocol handler (unpackaged) for the Shell Host.
# Run elevated optional; HKCU is enough for current user.
param(
  [string]$HostExe = "",
  [switch]$Unregister
)

$ErrorActionPreference = "Stop"
$key = "HKCU:\Software\Classes\agent-grid"

if ($Unregister) {
  if (Test-Path $key) {
    Remove-Item -Recurse -Force $key
    Write-Host "Unregistered agent-grid:// from HKCU"
  } else {
    Write-Host "Nothing to unregister"
  }
  exit 0
}

if (-not $HostExe) {
  $candidates = @(
    (Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\AgentGrid.Shell.WinUI.exe"),
    (Join-Path $env:LOCALAPPDATA "agent-grid\shell-winui\AgentGrid.Shell.WinUI.exe"),
    (Join-Path $PSScriptRoot "..\AgentGrid.Shell.Host\bin\Release\net8.0\AgentGrid.Shell.Host.exe"),
    (Join-Path $PSScriptRoot "..\AgentGrid.Shell.Host\bin\Debug\net8.0\AgentGrid.Shell.Host.exe")
  )
  foreach ($c in $candidates) {
    if (Test-Path $c) { $HostExe = $c; break }
  }
}
if (-not $HostExe -or -not (Test-Path $HostExe)) {
  throw "HostExe not found. Build WinUI first (scripts/win-build-winui.ps1) or pass -HostExe."
}
$HostExe = (Resolve-Path $HostExe).Path
$isWinUi = $HostExe -match 'WinUI'

New-Item -Path $key -Force | Out-Null
Set-ItemProperty -Path $key -Name "(default)" -Value "URL:Agent Grid Protocol"
Set-ItemProperty -Path $key -Name "URL Protocol" -Value ""

$cmdKey = Join-Path $key "shell\open\command"
New-Item -Path $cmdKey -Force | Out-Null
# WinUI AppInstance protocol activation receives the URI via activation args;
# also pass on argv for cold-start HandleDeepLinkAsync.
if ($isWinUi) {
  Set-ItemProperty -Path $cmdKey -Name "(default)" -Value "`"$HostExe`" `"%1`""
} else {
  Set-ItemProperty -Path $cmdKey -Name "(default)" -Value "`"$HostExe`" --deep-link `"%1`""
}

Write-Host "Registered agent-grid:// -> $HostExe"
Write-Host "Test: start agent-grid://server/local/session/demo"
