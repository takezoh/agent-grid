param(
  [string]$Configuration = "Debug",
  [string]$OutDir = ""
)

$ErrorActionPreference = "Stop"
$repo = (wsl.exe -e wslpath -w /workspace/agent-grid).Trim()
$localRoot = Join-Path $env:LOCALAPPDATA (
  "Temp\ag-workspace-src-" + [Guid]::NewGuid().ToString("N"))
if (-not $OutDir) {
  $OutDir = Join-Path $env:LOCALAPPDATA (
    "Temp\ag-workspace-build-" + [Guid]::NewGuid().ToString("N"))
}
$OutDir = [System.IO.Path]::GetFullPath($OutDir)

try {
  $localClients = Join-Path $localRoot "clients"
  New-Item -ItemType Directory -Force -Path $localClients | Out-Null
  foreach ($name in @("ui", "workspace")) {
    $source = Join-Path $repo "clients\$name"
    $destination = Join-Path $localClients $name
    & robocopy $source $destination /E /XD node_modules dist .git `
      /NFL /NDL /NJH /NJS /nc /ns /np | Out-Null
    if ($LASTEXITCODE -ge 8) { throw "robocopy $name failed: $LASTEXITCODE" }
  }

  $workspace = Join-Path $localClients "workspace"
  $ui = Join-Path $localClients "ui"
  Push-Location $workspace
  & npm.cmd ci
  if ($LASTEXITCODE -ne 0) { throw "Workspace npm ci failed" }
  & node.exe "node_modules\electron\install.js"
  if ($LASTEXITCODE -ne 0) { throw "Electron runtime install failed" }
  Push-Location $ui
  & npm.cmd ci
  if ($LASTEXITCODE -ne 0) { throw "UI npm ci failed" }
  Pop-Location
  & npm.cmd run build
  if ($LASTEXITCODE -ne 0) { throw "Workspace build failed" }
  Pop-Location

  if (Test-Path $OutDir) { Remove-Item -Recurse -Force $OutDir }
  $electronDist = Join-Path $workspace "node_modules\electron\dist"
  if (-not (Test-Path (Join-Path $electronDist "electron.exe"))) {
    throw "Electron Windows runtime is missing"
  }
  New-Item -ItemType Directory -Force -Path $OutDir | Out-Null
  Copy-Item (Join-Path $electronDist "*") $OutDir -Recurse -Force

  $electronExe = Join-Path $OutDir "electron.exe"
  $workspaceExe = Join-Path $OutDir "agent-grid-workspace.exe"
  Move-Item $electronExe $workspaceExe

  $appDir = Join-Path $OutDir "resources\app"
  New-Item -ItemType Directory -Force -Path $appDir | Out-Null
  Copy-Item (Join-Path $workspace "dist\*") $appDir -Recurse -Force

  $appPackage = [ordered]@{
    name = "agent-grid-workspace"
    version = "0.0.0"
    private = $true
    type = "module"
    main = "main/electron-entry.js"
  } | ConvertTo-Json
  [System.IO.File]::WriteAllText(
    (Join-Path $appDir "package.json"),
    $appPackage + [Environment]::NewLine,
    [System.Text.UTF8Encoding]::new($false))

  if (-not (Test-Path $workspaceExe)) { throw "Workspace EXE was not produced" }
  if (-not (Test-Path (Join-Path $appDir "ui\index.html"))) {
    throw "Bundled UI was not produced"
  }
  Write-Host "Built Workspace ($Configuration)"
  Write-Host "EXE: $workspaceExe"
  Write-Output "OUT_DIR=$OutDir"
}
finally {
  while ((Get-Location).Path.StartsWith($localRoot, [StringComparison]::OrdinalIgnoreCase)) {
    Pop-Location
  }
  if (Test-Path $localRoot) { Remove-Item -Recurse -Force $localRoot }
}
