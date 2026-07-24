namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Runner for configured remote servers. Health/adoption is supported, but the
/// desktop must never try to start or stop a process on the remote host.
/// </summary>
public sealed class ConnectOnlyDaemonRunner : IDaemonRunner
{
    public Task<SpawnResult> SpawnAsync(CancellationToken ct = default) =>
        Task.FromResult(new SpawnResult(
            SpawnResultKind.Failed,
            "connect-only server is unavailable"));

    public Task ShutdownAsync(CancellationToken ct = default) => Task.CompletedTask;
}
