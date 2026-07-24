# Build AgentGrid.Shell.WinUI (unpackaged) on local NTFS after robocopy from WSL.
$ErrorActionPreference = "Stop"
$dotnet = Join-Path $env:LOCALAPPDATA "Microsoft\dotnet\dotnet.exe"
if (-not (Test-Path $dotnet)) { $dotnet = "C:\Program Files\dotnet\dotnet.exe" }
$env:DOTNET_ROOT = Split-Path $dotnet
$env:PATH = "$env:DOTNET_ROOT;$env:PATH"
$env:DOTNET_CLI_TELEMETRY_OPTOUT = "1"

$repo = (wsl.exe -e wslpath -w /workspace/agent-grid).Trim()
$shell = Join-Path $repo "clients\windows-shell"
$local = Join-Path $env:LOCALAPPDATA "Temp\ag-shell-src"
Write-Host "Sync $shell -> $local"
if (Test-Path $local) { Remove-Item -Recurse -Force $local }
& robocopy $shell $local /E /XD bin obj .git /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
if ($LASTEXITCODE -ge 8) { throw "robocopy failed: $LASTEXITCODE" }

Set-Location $local
Write-Host "dotnet build WinUI (x64, WindowsAppSDK self-contained)..."
& $dotnet build AgentGrid.Shell.WinUI\AgentGrid.Shell.WinUI.csproj `
  -c Debug -p:Platform=x64 -p:RuntimeIdentifier=win-x64 `
  -p:WindowsAppSDKSelfContained=true -p:SelfContained=true --nologo
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

$exe = Join-Path $local "AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\win-x64\AgentGrid.Shell.WinUI.exe"
if (-not (Test-Path $exe)) {
  # Fallback older layout without rid folder
  $exe = Join-Path $local "AgentGrid.Shell.WinUI\bin\x64\Debug\net8.0-windows10.0.19041.0\AgentGrid.Shell.WinUI.exe"
}
if (Test-Path $exe) {
  Write-Host "EXE: $exe"
  $boot = Join-Path (Split-Path $exe) "Microsoft.WindowsAppRuntime.Bootstrap.dll"
  Write-Host "Bootstrap.dll present: $(Test-Path $boot)"
  & (Join-Path $PSScriptRoot "assert-winui-layout.ps1") -OutDir (Split-Path $exe)
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} else {
  Write-Host "WARN: WinUI exe not found under bin"
  exit 1
}

# Unit tests on AnyCPU Debug (no RID) — separate from WinUI self-contained output
& $dotnet test AgentGrid.Shell.sln --nologo --filter "FullyQualifiedName!~WinUI&FullyQualifiedName!~E2E"
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Optional process smoke (skip with AG_WINUI_SKIP_SMOKE=1)
if ($env:AG_WINUI_SKIP_SMOKE -ne "1") {
  Write-Host "Launch smoke..."
  & (Join-Path $PSScriptRoot "launch-smoke.ps1") -Exe $exe -SmokeSeconds 4
  exit $LASTEXITCODE
}
exit 0
