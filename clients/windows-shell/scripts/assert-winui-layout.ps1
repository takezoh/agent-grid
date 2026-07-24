# T1-style layout contract: self-contained WinUI output must ship Bootstrap natives.
# Fails with a message matching the user-visible Runtime 1.6 / MSIX failure mode.
param(
  [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"

if (-not $OutDir) {
  $candidates = @(
    (Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\win-x64"),
    (Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0")
  )
  foreach ($c in $candidates) {
    if (Test-Path (Join-Path $c "AgentGrid.Shell.WinUI.exe")) { $OutDir = $c; break }
  }
}

if (-not $OutDir -or -not (Test-Path $OutDir)) {
  Write-Error "WinUI out dir not found. Run win-build-winui.ps1 first."
  exit 3
}

Write-Host "Checking layout: $OutDir"
$required = @(
  "AgentGrid.Shell.WinUI.exe",
  "Microsoft.WindowsAppRuntime.Bootstrap.dll",
  "Microsoft.WindowsAppRuntime.Bootstrap.Net.dll",
  "Microsoft.ui.xaml.dll",
  "Microsoft.WindowsAppRuntime.dll"
)
$missing = @()
foreach ($n in $required) {
  if (-not (Test-Path (Join-Path $OutDir $n))) { $missing += $n }
}

if ($missing.Count -gt 0) {
  Write-Host "FAIL: incomplete self-contained layout (would surface as:"
  Write-Host "  'This application requires the Windows App Runtime Version 1.6, MSIX package...')"
  $missing | ForEach-Object { Write-Host "  missing: $_" }
  exit 1
}

Write-Host "PASS: self-contained WinUI layout"
exit 0
