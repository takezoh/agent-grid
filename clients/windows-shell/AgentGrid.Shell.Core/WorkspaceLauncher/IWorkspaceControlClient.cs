namespace AgentGrid.Shell.Core.WorkspaceLauncher;

/// <summary>
/// Boundary-1 adapter on the Shell side: named-pipe client + spawn-on-miss with bounded backoff
/// (FR-B1-03, contract-b1-b2-launch-ordering).
/// </summary>
public interface IWorkspaceControlClient
{
    Task<ControlReply> SendAsync(ControlEnvelope envelope, CancellationToken ct = default);
}

public interface IWorkspaceProcessLauncher
{
    /// <summary>Spawn the Workspace executable if not already running.</summary>
    Task SpawnAsync(CancellationToken ct = default);
}

/// <summary>
/// Connects to the control endpoint; on miss, spawns Workspace and retries with bounded backoff.
/// </summary>
public sealed class WorkspaceLauncherService
{
    private readonly IWorkspaceControlClient _client;
    private readonly IWorkspaceProcessLauncher _process;
    private readonly int _maxAttempts;
    private readonly TimeSpan _initialBackoff;

    public WorkspaceLauncherService(
        IWorkspaceControlClient client,
        IWorkspaceProcessLauncher process,
        int maxAttempts = 5,
        TimeSpan? initialBackoff = null)
    {
        _client = client;
        _process = process;
        _maxAttempts = Math.Max(1, maxAttempts);
        _initialBackoff = initialBackoff ?? TimeSpan.FromMilliseconds(100);
    }

    public async Task<ControlReply> OpenSessionAsync(string sessionId, CancellationToken ct = default)
    {
        var env = new ControlEnvelope { Op = "openSession", Id = sessionId };
        return await SendWithLaunchRetryAsync(env, ct).ConfigureAwait(false);
    }

    public async Task<ControlReply> ActivateAsync(CancellationToken ct = default)
    {
        var env = new ControlEnvelope { Op = "activate" };
        return await SendWithLaunchRetryAsync(env, ct).ConfigureAwait(false);
    }

    private async Task<ControlReply> SendWithLaunchRetryAsync(
        ControlEnvelope env,
        CancellationToken ct)
    {
        Exception? last = null;
        var delay = _initialBackoff;
        for (var attempt = 0; attempt < _maxAttempts; attempt++)
        {
            try
            {
                return await _client.SendAsync(env, ct).ConfigureAwait(false);
            }
            catch (Exception ex) when (ex is IOException or TimeoutException or InvalidOperationException)
            {
                last = ex;
                if (attempt == 0)
                    await _process.SpawnAsync(ct).ConfigureAwait(false);
                await Task.Delay(delay, ct).ConfigureAwait(false);
                delay = TimeSpan.FromMilliseconds(Math.Min(delay.TotalMilliseconds * 2, 2000));
            }
        }

        return ControlReply.Fail(last?.Message ?? "workspace launch failed");
    }
}
