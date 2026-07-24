# Windows entry for the WSL detach spike. Invokes the in-distro bash script.
param(
  [string]$Distro = "Ubuntu-22.04",
  [string]$ServerPath = "/workspace/agent-grid/server",
  [int]$Port = 18443,
  [int]$WaitSeconds = 6
)

$ErrorActionPreference = "Stop"
$script = "/workspace/agent-grid/clients/windows-shell/scripts/wsl-detach-spike.sh"

Write-Host "Invoking spike in $Distro via wsl.exe..."
& wsl.exe -d $Distro --cd / -- env `
  "SERVER_PATH=$ServerPath" `
  "PORT=$Port" `
  "WAIT_SECONDS=$WaitSeconds" `
  bash --noprofile --norc "$script"
exit $LASTEXITCODE
