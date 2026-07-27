param(
  [Parameter(Mandatory = $true)][string]$ConfigDir,
  [Parameter(Mandatory = $true)][string]$GatewayUrl,
  [string]$WorkspaceExecutable = "agent-grid-workspace",
  [string]$Token = ""
)

$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path $ConfigDir | Out-Null
$auth = if ($Token) {
  @{ mode = "bearer"; token = $Token }
} else {
  @{ mode = "none" }
}

@{
  schema_version = 2
  servers = @(@{
    display_name = "Test"
    enabled = $true
    url = $GatewayUrl
    auth = $auth
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
