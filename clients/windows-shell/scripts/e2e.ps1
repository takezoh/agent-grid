# Windows-side entry for Shell e2e (client tests only).
# Delegates to the WSL harness so run-dev fixture logic stays one place.
param(
  [string]$GatewayUrl = "http://127.0.0.1:8443",
  [switch]$SkipUnit,
  [switch]$StartRunDev
)

$ErrorActionPreference = "Stop"

$parts = @("/workspace/agent-grid/clients/windows-shell/scripts/e2e.sh")
if ($StartRunDev) { $parts += "--start-run-dev" }
if ($SkipUnit) { $parts += "--skip-unit" }
$parts += @("--gateway-url", $GatewayUrl)

Write-Host "Invoking WSL e2e harness: $($parts -join ' ')"
& wsl.exe -d Ubuntu-22.04 --cd /workspace/agent-grid -- bash --noprofile --norc @parts
exit $LASTEXITCODE
