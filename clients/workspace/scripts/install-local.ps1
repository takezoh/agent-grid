param(
  [string]$Configuration = "Release"
)

$ErrorActionPreference = "Stop"
$stage = Join-Path $env:LOCALAPPDATA (
  "Temp\ag-workspace-install-" + [Guid]::NewGuid().ToString("N"))
$destination = Join-Path $env:LOCALAPPDATA "agent-grid\workspace"

try {
  & (Join-Path $PSScriptRoot "build-windows.ps1") `
    -Configuration $Configuration -OutDir $stage
  if ($LASTEXITCODE -ne 0) { throw "Workspace build failed" }

  Get-Process "agent-grid-workspace" -ErrorAction SilentlyContinue |
    Stop-Process -Force
  if (Test-Path $destination) { Remove-Item -Recurse -Force $destination }
  New-Item -ItemType Directory -Force -Path $destination | Out-Null
  & robocopy $stage $destination /E /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
  if ($LASTEXITCODE -ge 8) { throw "Workspace install copy failed: $LASTEXITCODE" }

  $exe = Join-Path $destination "agent-grid-workspace.exe"
  if (-not (Test-Path $exe)) { throw "Installed Workspace EXE is missing" }
  Write-Host "Installed Workspace to $destination"
  Write-Host "Run: & '$exe'"
}
finally {
  if (Test-Path $stage) { Remove-Item -Recurse -Force $stage }
}
