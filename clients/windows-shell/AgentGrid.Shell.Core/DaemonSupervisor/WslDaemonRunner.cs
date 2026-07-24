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
    private readonly Func<string, string, Task<int>> _run;
    private readonly Func<Task> _shutdown;

    public WslDaemonRunner(
        string distro,
        string serverPath,
        int port,
        string tokenFileInWsl = "~/.agent-grid/gateway-token",
        Func<string, string, Task<int>>? run = null,
        Func<Task>? shutdown = null)
    {
        _distro = distro;
        _serverPath = serverPath;
        _port = port;
        _tokenFileInWsl = tokenFileInWsl;
        _run = run ?? DefaultRunAsync;
        _shutdown = shutdown ?? (() => Task.CompletedTask);
    }

    public string BuildDetachCommand() =>
        $"setsid nohup {_serverPath} -addr 127.0.0.1:{_port} -token-file {_tokenFileInWsl} " +
        $">/tmp/agent-grid-server.log 2>&1 </dev/null &";

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
        // Production path uses System.Diagnostics.Process; kept out of Core
        // default so Linux unit tests never spawn wsl.exe.
        throw new PlatformNotSupportedException(
            "Inject a process runner; default is Windows-only.");
    }
}
