[CmdletBinding(SupportsShouldProcess = $true)]
param(
    [ValidateSet("Install", "Update", "Uninstall")]
    [string]$Action = "Install",
    [string]$SupervisorExe,
    [System.Management.Automation.PSCredential]$Credential,
    [string]$Distribution,
    [string]$LinuxUser,
    [string]$ServerExecutable,
    [string]$DataDir,
    [string]$Addr = "127.0.0.1:8443",
    [ValidateSet("tls", "insecure")]
    [string]$TLSMode = "tls",
    [string]$HealthURL,
    [string]$LogPath,
    [switch]$NoStart
)

$ErrorActionPreference = "Stop"
$ServiceName = "AgentGridWSLHost"

function Assert-Administrator {
    $identity = [Security.Principal.WindowsIdentity]::GetCurrent()
    $principal = [Security.Principal.WindowsPrincipal]::new($identity)
    if (-not $principal.IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
        throw "Run this script from an elevated PowerShell session."
    }
}

function Get-AgentGridService {
    Get-Service -Name $ServiceName -ErrorAction SilentlyContinue
}

function Stop-AgentGridService {
    $service = Get-AgentGridService
    if ($null -ne $service -and $service.Status -ne "Stopped") {
        Stop-Service -Name $ServiceName
        $service.WaitForStatus("Stopped", [TimeSpan]::FromSeconds(30))
    }
}

function Remove-AgentGridService {
    if ($null -eq (Get-AgentGridService)) {
        return
    }
    & sc.exe delete $ServiceName | Out-Host
    if ($LASTEXITCODE -ne 0) {
        throw "sc.exe delete failed with exit code $LASTEXITCODE"
    }
    for ($attempt = 0; $attempt -lt 50 -and $null -ne (Get-AgentGridService); $attempt++) {
        Start-Sleep -Milliseconds 100
    }
    if ($null -ne (Get-AgentGridService)) {
        throw "Service deletion did not complete: $ServiceName"
    }
}

function Assert-InstallParameters {
    $required = @{
        SupervisorExe = $SupervisorExe
        Distribution = $Distribution
        LinuxUser = $LinuxUser
        ServerExecutable = $ServerExecutable
        DataDir = $DataDir
    }
    foreach ($entry in $required.GetEnumerator()) {
        if ([string]::IsNullOrWhiteSpace([string]$entry.Value)) {
            throw "-$($entry.Key) is required for $Action."
        }
    }
    if ($null -eq $Credential) {
        $script:Credential = Get-Credential -Message "Credentials for the Windows account that owns the WSL distribution"
    }
    $script:SupervisorExe = (Resolve-Path -LiteralPath $SupervisorExe).Path
}

function Register-AgentGridSnapshot {
    $arguments = @(
        "register",
        "-windows-identity", $Credential.UserName,
        "-distribution", $Distribution,
        "-linux-user", $LinuxUser,
        "-server-executable", $ServerExecutable,
        "-data-dir", $DataDir,
        "-addr", $Addr,
        "-tls-mode", $TLSMode
    )
    if (-not [string]::IsNullOrWhiteSpace($HealthURL)) {
        $arguments += @("-health-url", $HealthURL)
    }
    if (-not [string]::IsNullOrWhiteSpace($LogPath)) {
        $arguments += @("-log-path", $LogPath)
    }
    & $SupervisorExe @arguments
    if ($LASTEXITCODE -ne 0) {
        throw "Supervisor registration failed with exit code $LASTEXITCODE"
    }
}

function New-AgentGridService {
    $binaryPath = '"{0}"' -f $SupervisorExe
    New-Service `
        -Name $ServiceName `
        -BinaryPathName $binaryPath `
        -Credential $Credential `
        -StartupType Automatic `
        -DisplayName "Agent Grid WSL Host" `
        -Description "Supervises the Agent Grid host server inside the registration-bound WSL distribution."
}

Assert-Administrator

if ($Action -eq "Uninstall") {
    if ($PSCmdlet.ShouldProcess($ServiceName, "stop and remove Windows Service")) {
        Stop-AgentGridService
        Remove-AgentGridService
    }
    return
}

Assert-InstallParameters
$existing = Get-AgentGridService
if ($Action -eq "Install" -and $null -ne $existing) {
    throw "Service $ServiceName already exists. Use -Action Update."
}
if ($Action -eq "Update" -and $null -eq $existing) {
    throw "Service $ServiceName does not exist. Use -Action Install."
}

if ($PSCmdlet.ShouldProcess($ServiceName, "$Action registration snapshot and Windows Service")) {
    Stop-AgentGridService
    Register-AgentGridSnapshot
    if ($Action -eq "Update") {
        Remove-AgentGridService
    }
    New-AgentGridService
    if (-not $NoStart) {
        Start-Service -Name $ServiceName
    }
    Get-Service -Name $ServiceName
}
