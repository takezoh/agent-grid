# Launch WinUI and verify real startup (not "still running while MessageBox is up").
#
# Success criteria (all):
#   1) layout has Bootstrap + Microsoft.ui.xaml.dll next to EXE
#   2) process still alive after SmokeSeconds
#   3) winui-startup-ok.txt exists (bootstrap_ok / self-contained-skip-mdd)
#   4) MainWindowTitle does not contain "startup failed" / "could not be started"
#   5) winui-startup-error.txt is absent
#
# Failure dumps structured log (SoT for Runtime 1.6 / MSIX-class errors).
param(
  [string]$Exe = "",
  [int]$SmokeSeconds = 5
)

$ErrorActionPreference = "Stop"

function Find-WinUiExe {
  $candidates = @(
    (Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\win-x64\AgentGrid.Shell.WinUI.exe"),
    (Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src\AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\AgentGrid.Shell.WinUI.exe"),
    (Join-Path $env:LOCALAPPDATA "agent-grid\shell-winui\AgentGrid.Shell.WinUI.exe")
  )
  foreach ($c in $candidates) {
    if (Test-Path $c) { return (Resolve-Path $c).Path }
  }
  return $null
}

function Dump-Logs($paths) {
  foreach ($f in $paths) {
    if (Test-Path $f) {
      Write-Host "==== $f ===="
      Get-Content $f -Raw
    }
  }
}

if (-not $Exe) { $Exe = Find-WinUiExe }
if (-not $Exe -or -not (Test-Path $Exe)) {
  Write-Error "WinUI exe not found. Run scripts/win-build-winui.ps1 first."
  exit 3
}

$dir = Split-Path $Exe
Write-Host "EXE: $Exe"

$required = @(
  "Microsoft.WindowsAppRuntime.Bootstrap.dll",
  "Microsoft.WindowsAppRuntime.Bootstrap.Net.dll",
  "Microsoft.ui.xaml.dll",
  "Microsoft.WindowsAppRuntime.dll"
)
$missing = @()
foreach ($n in $required) {
  if (-not (Test-Path (Join-Path $dir $n))) { $missing += $n }
}
if ($missing.Count -gt 0) {
  Write-Host "FAIL layout: missing next to EXE (causes Runtime 1.6 / MSIX dialog):"
  $missing | ForEach-Object { Write-Host "  - $_" }
  exit 3
}
Write-Host "Layout OK"

$log = Join-Path $env:LOCALAPPDATA "agent-grid\logs\winui-startup-error.txt"
$ok = Join-Path $env:LOCALAPPDATA "agent-grid\logs\winui-startup-ok.txt"
$besideErr = Join-Path $dir "winui-startup-error.txt"
$besideOk = Join-Path $dir "winui-startup-ok.txt"
Remove-Item $log,$ok,$besideErr,$besideOk -ErrorAction SilentlyContinue

Get-Process AgentGrid.Shell.WinUI -ErrorAction SilentlyContinue | Stop-Process -Force -ErrorAction SilentlyContinue
Start-Sleep -Milliseconds 500

$psi = New-Object System.Diagnostics.ProcessStartInfo
$psi.FileName = $Exe
$psi.WorkingDirectory = $dir
$psi.UseShellExecute = $false
$psi.RedirectStandardError = $true
$psi.RedirectStandardOutput = $true
foreach ($key in [System.Environment]::GetEnvironmentVariables().Keys) {
  if (-not $psi.Environment.ContainsKey([string]$key)) {
    try { $psi.Environment[[string]$key] = [System.Environment]::GetEnvironmentVariable([string]$key) } catch {}
  }
}
$psi.Environment["AG_WINUI_STARTUP_LOG"] = $log
$psi.Environment["AG_WINUI_STARTUP_OK"] = $ok
$psi.Environment["AG_WINUI_NO_MSGBOX"] = "1"
$psi.Environment["AG_NO_AUTH"] = "1"

$p = New-Object System.Diagnostics.Process
$p.StartInfo = $psi
[void]$p.Start()
Write-Host "started pid=$($p.Id)"

$deadline = (Get-Date).AddSeconds($SmokeSeconds)
$badTitle = $false
$title = ""
while ((Get-Date) -lt $deadline) {
  Start-Sleep -Milliseconds 250
  if ($p.HasExited) { break }
  try {
    $proc = Get-Process -Id $p.Id -ErrorAction Stop
    $title = [string]$proc.MainWindowTitle
    if ($title -match "startup failed|could not be started|Windows App Runtime") {
      $badTitle = $true
      break
    }
  } catch { break }
}

$stdout = ""
$stderr = ""
if ($p.HasExited) {
  $stdout = $p.StandardOutput.ReadToEnd()
  $stderr = $p.StandardError.ReadToEnd()
}

$hasOk = (Test-Path $ok) -or (Test-Path $besideOk)
$hasErr = (Test-Path $log) -or (Test-Path $besideErr)

if ($p.HasExited) {
  Write-Host "Process exited code=$($p.ExitCode)"
  if ($stdout) { Write-Host "STDOUT:`n$stdout" }
  if ($stderr) { Write-Host "STDERR:`n$stderr" }
  Dump-Logs @($log, $besideErr, $ok, $besideOk)
  Write-Host "FAIL: process exited before smoke window"
  exit 2
}

if ($badTitle) {
  Write-Host "FAIL: error window title=[$title]"
  Dump-Logs @($log, $besideErr)
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  exit 2
}

if ($hasErr -and -not $hasOk) {
  Write-Host "FAIL: startup error log present without ok marker"
  Dump-Logs @($log, $besideErr)
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  exit 2
}

if (-not $hasOk) {
  Write-Host "FAIL: no winui-startup-ok.txt (bootstrap did not report success)"
  Dump-Logs @($log, $besideErr)
  try {
    $proc = Get-Process -Id $p.Id -ErrorAction SilentlyContinue
    Write-Host "title=[$($proc.MainWindowTitle)]"
  } catch {}
  Stop-Process -Id $p.Id -Force -ErrorAction SilentlyContinue
  exit 2
}

Write-Host "PASS: process alive pid=$($p.Id) after ${SmokeSeconds}s; ok marker present"
Write-Host "ok marker:"
Get-Content $(if (Test-Path $ok) { $ok } else { $besideOk })
Write-Host "title=[$title]"
# Leave process running so the user can interact; smoke already verified.
Write-Host "Process left running for interactive use (pid=$($p.Id))"
exit 0
