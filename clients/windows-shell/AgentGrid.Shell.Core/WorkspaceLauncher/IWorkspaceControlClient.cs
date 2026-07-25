using AgentGrid.Shell.Core.SessionIdentity;

namespace AgentGrid.Shell.Core.WorkspaceLauncher;

/// <summary>
/// Boundary-1 adapter on the Shell side: named-pipe client + spawn-on-miss with bounded backoff
/// (FR-B1-03, contract-b1-b2-launch-ordering).
/// </summary>
/// <remarks>
/// Exception contract (debug/workspace-not-launched): implementations connect-timeout via an
/// internal linked <see cref="CancellationTokenSource"/> derived from the caller's <c>ct</c>, so
/// a connect-timeout and a caller-initiated cancellation both surface as
/// <see cref="OperationCanceledException"/> to <see cref="SendAsync"/> callers -- the CLR gives no
/// distinct exception type for "pipe server not listening yet" versus "caller gave up". Consumers
/// (see <see cref="WorkspaceLauncherService.SendWithLaunchRetryAsync"/>) MUST distinguish the two
/// by provenance -- inspecting whether the caller's own <c>ct</c> is the one that was canceled --
/// not by exception type alone, or they will either fail to spawn on a cold-start connect timeout
/// or spawn a workspace process after the caller already gave up.
/// </remarks>
public interface IWorkspaceControlClient
{
    /// <summary>
    /// Sends <paramref name="envelope"/> over the control transport and returns the reply.
    /// </summary>
    /// <param name="onConnectedServerProcessId">
    /// Foreground-transfer hook (debug-run-workspace-not-activated): if the transport can
    /// determine the process id of the server it just connected to, it invokes this callback
    /// synchronously with that PID -- BEFORE <paramref name="envelope"/> is written to the wire.
    /// This lets a caller (see <see cref="WorkspaceLauncherService"/>) grant the target process OS
    /// foreground permission (e.g. via <c>IWin32InteropService.AllowSetForegroundWindow</c>) before
    /// the target's own <c>SetForegroundWindow</c>/<c>focus()</c> call runs in response to this
    /// message; otherwise Windows' foreground-lock restriction demotes that call to a taskbar
    /// flash, because the target is not the *caller's* (Shell's) foreground process at click time.
    /// Not invoked when the transport cannot determine a server process id (e.g. the
    /// Unix-domain-socket path used by WSL/Linux tests) -- callers MUST NOT assume it fires.
    /// </param>
    Task<ControlReply> SendAsync(
        ControlEnvelope envelope,
        CancellationToken ct = default,
        Action<int>? onConnectedServerProcessId = null);
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
    private readonly Action<int>? _onForegroundTransferEligible;

    /// <param name="onForegroundTransferEligible">
    /// Composition-layer delegate (decision authority stays outside Core -- Core does not
    /// reference <c>IWin32InteropService</c>/<c>AgentGrid.Shell.Platform</c>, avoiding a
    /// Core&#8594;Platform circular reference against Platform's existing Platform&#8594;Core
    /// reference): invoked with an already-connected control-transport server's process id
    /// (see <see cref="IWorkspaceControlClient.SendAsync"/>'s <c>onConnectedServerProcessId</c>
    /// parameter) so a composition root (e.g. <c>ShellCompositionRoot.Build</c>) can wire it to
    /// <c>win32.AllowSetForegroundWindow(pid)</c> -- debug-run-workspace-not-activated: without
    /// this, the pipe-success path (workspace already running) never asks Windows for permission
    /// to foreground the workspace, and Electron's own <c>focus()</c> is demoted to a taskbar
    /// flash. Shared by <see cref="OpenSessionAsync"/> and <see cref="ActivateAsync"/> because
    /// both funnel through <see cref="SendWithLaunchRetryAsync"/>.
    /// </param>
    public WorkspaceLauncherService(
        IWorkspaceControlClient client,
        IWorkspaceProcessLauncher process,
        int maxAttempts = 5,
        TimeSpan? initialBackoff = null,
        Action<int>? onForegroundTransferEligible = null)
    {
        _client = client;
        _process = process;
        _maxAttempts = Math.Max(1, maxAttempts);
        _initialBackoff = initialBackoff ?? TimeSpan.FromMilliseconds(100);
        _onForegroundTransferEligible = onForegroundTransferEligible;
    }

    public async Task<ControlReply> OpenSessionAsync(
        ServerSessionId session,
        CancellationToken ct = default)
    {
        session.Validate();
        var env = new ControlEnvelope
        {
            Op = "openSession",
            ServerId = session.ServerId,
            SessionId = session.SessionId,
        };
        return await SendWithLaunchRetryAsync(env, ct).ConfigureAwait(false);
    }

    public async Task<ControlReply> ActivateAsync(CancellationToken ct = default)
    {
        var env = new ControlEnvelope { Op = "activate" };
        return await SendWithLaunchRetryAsync(env, ct).ConfigureAwait(false);
    }

    /// <summary>
    /// Sends <paramref name="env"/>, spawning the Workspace process and retrying with bounded
    /// backoff when the control pipe is unreachable (contract-b1-b2-launch-ordering).
    /// </summary>
    /// <remarks>
    /// Spawn-fallback condition (debug/workspace-not-launched): a connect failure is treated as
    /// "workspace not started yet" -- and triggers spawn/retry -- when it is an
    /// <see cref="IOException"/>, <see cref="TimeoutException"/>, <see cref="InvalidOperationException"/>,
    /// or an <see cref="OperationCanceledException"/> whose provenance is NOT the caller's own
    /// <paramref name="ct"/> (i.e. <c>!ct.IsCancellationRequested</c> at catch time). The client's
    /// internal connect-timeout CancellationTokenSource is linked from <paramref name="ct"/>, so an
    /// <see cref="OperationCanceledException"/> is ambiguous by type alone; checking
    /// <c>ct.IsCancellationRequested</c> is what tells apart "our own connect-timeout budget
    /// elapsed" (spawn) from "the caller canceled the token it handed us" (propagate, do not
    /// spawn -- the caller no longer wants this operation to continue).
    /// </remarks>
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
                return await _client.SendAsync(env, ct, _onForegroundTransferEligible)
                    .ConfigureAwait(false);
            }
            catch (Exception ex) when (
                ex is IOException or TimeoutException or InvalidOperationException
                || (ex is OperationCanceledException && !ct.IsCancellationRequested))
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
