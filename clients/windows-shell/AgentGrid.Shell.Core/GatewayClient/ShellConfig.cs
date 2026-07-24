namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Shell configuration loaded from %APPDATA%\agent-grid\shell.json (personal use).
/// </summary>
public sealed class ShellConfig
{
    public string BaseUrl { get; init; } = "http://127.0.0.1:8443";
    public string WebOrigin { get; init; } = "http://127.0.0.1:8080";
    public int Port { get; init; } = 8443;
    public string WslDistro { get; init; } = "Ubuntu";
    public string ServerPathInWsl { get; init; } = "~/dev/agent-grid/server";
    public string TokenUncPath { get; init; } =
        @"\\wsl$\Ubuntu\home\user\.agent-grid\gateway-token";
    public string WorkspaceExecutable { get; init; } = "";
    /// <summary>When true, only adopt — never spawn (remote Linux host).</summary>
    public bool AdoptOnly { get; init; }
}
