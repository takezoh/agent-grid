param(
  [Parameter(Mandatory = $true)][string]$ConfigDir,
  [Parameter(Mandatory = $true)][string]$GatewayUrl,
  [string]$WorkspaceExecutable = "agent-grid-workspace",
  [string]$TokenPath = ""
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
if (-not $TokenPath) {
  $TokenPath = Join-Path $ConfigDir "test-gateway-token"
  Set-Content -Path $TokenPath -Value "test-no-auth-token" -NoNewline
}

@{
  schema_version = 1
  servers = @(@{
    id = "test"
    display_name = "Test"
    enabled = $true
    url = $GatewayUrl
    token_path = $TokenPath
    launch = @{ mode = "connect_only" }
  })
} | ConvertTo-Json -Depth 8 | Set-Content (Join-Path $ConfigDir "servers.json")

@{
  schema_version = 1
  theme = "default"
  density = "comfortable"
  font_scale = 1.0
} | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $ConfigDir "appearance.json")

@{
  schema_version = 1
  workspace_executable = $WorkspaceExecutable
  health_poll_interval_seconds = 1
} | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $ConfigDir "shell.json")

@{
  schema_version = 1
  idle_quit_seconds = 30
  default_window = @{ width = 1280; height = 800 }
} | ConvertTo-Json -Depth 4 | Set-Content (Join-Path $ConfigDir "workspace.json")

Write-Output $ConfigDir
