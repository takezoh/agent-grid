namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Real Runner for WSL-hosted server. Process APIs are injectable for tests.
/// Detach uses setsid+nohup per adr-20260724-boundary-3-wsl-detach-spike.
/// </summary>
public sealed class WslDaemonRunner : IDaemonRunner
{
    private readonly string _distro;
    private readonly string _serverPath;
    private readonly int _port;
    private readonly string _tokenFileInWsl;
    private readonly string _dataDirInWsl;
    private readonly Func<string, string, Task<int>> _run;
    private readonly Func<Task> _shutdown;

    public WslDaemonRunner(
        string distro,
        string serverPath,
        int port,
        string tokenFileInWsl = "~/.agent-grid/gateway-token",
        string dataDirInWsl = "/tmp/agent-grid-data",
        Func<string, string, Task<int>>? run = null,
        Func<Task>? shutdown = null)
    {
        _distro = distro;
        _serverPath = serverPath;
        _port = port;
        _tokenFileInWsl = tokenFileInWsl;
        _dataDirInWsl = dataDirInWsl;
        _run = run ?? DefaultRunAsync;
        _shutdown = shutdown ?? (() => Task.CompletedTask);
    }

    /// <summary>
    /// setsid+nohup candidate (adr-20260724-boundary-3-wsl-detach-spike).
    /// Uses explicit -data-dir so server.log is not forced under a possibly
    /// non-writable ~/.agent-grid (personal machines / sandbox homes).
    /// -insecure: loopback plain HTTP for local WSL supervision.
    /// </summary>
    public string BuildDetachCommand() =>
        $"mkdir -p {_dataDirInWsl} && " +
        $"setsid nohup {_serverPath} -data-dir {_dataDirInWsl} -addr 127.0.0.1:{_port} " +
        $"-token-file {_tokenFileInWsl} -insecure " +
        $">/tmp/agent-grid-server.log 2>&1 </dev/null & echo $!";
    public async Task<SpawnResult> SpawnAsync(CancellationToken ct = default)
    {
        try
        {
            var code = await _run("wsl.exe", $"-d {_distro} -- bash -lc '{BuildDetachCommand()}'")
                .WaitAsync(ct)
                .ConfigureAwait(false);
            // wsl.exe returns quickly after launching the backgrounded shell command.
            if (code != 0)
                return new SpawnResult(SpawnResultKind.Failed, $"wsl exit {code}");
            return new SpawnResult(SpawnResultKind.Started);
        }
        catch (Exception ex)
        {
            return new SpawnResult(SpawnResultKind.Failed, ex.Message);
        }
    }

    public Task ShutdownAsync(CancellationToken ct = default) => _shutdown();

    private static Task<int> DefaultRunAsync(string fileName, string args)
    {
        // Windows host (WinUI Shell via PowerShell/cmd): real Process.Start.
        // Non-Windows unit tests inject a fake; avoid accidental wsl.exe spawn.
        if (!OperatingSystem.IsWindows())
        {
            throw new PlatformNotSupportedException(
                "Default WslDaemonRunner process spawn is Windows-only; inject a runner for tests.");
        }
        return ProcessRunner.RunAsync(fileName, args);
    }
}
