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
Write-Host "dotnet build WinUI (x64)..."
& $dotnet build AgentGrid.Shell.WinUI\AgentGrid.Shell.WinUI.csproj -c Debug -p:Platform=x64 --nologo
if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }

# Also keep Core/Platform tests green on the same tree
& $dotnet test AgentGrid.Shell.sln --nologo --filter "FullyQualifiedName!~WinUI"
exit $LASTEXITCODE
