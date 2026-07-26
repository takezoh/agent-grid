namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Client-side connection monitor driven solely by gateway probes.
/// </summary>
public sealed class DaemonSupervisorService
{
    private readonly IDaemonHealthProbe _probe;
    private readonly ISurfaceResubscriber? _resubscriber;
    private readonly object _gate = new();
    private DaemonSnapshot _snapshot = DaemonSnapshot.Initial;

    public DaemonSupervisorService(
        IDaemonHealthProbe probe,
        ISurfaceResubscriber? resubscriber = null)
    {
        _probe = probe;
        _resubscriber = resubscriber;
    }

    public DaemonSnapshot Snapshot
    {
        get { lock (_gate) return _snapshot; }
    }

    /// <summary>
    /// Fires on whatever thread resumes the async effect that produced the
    /// transition — a thread-pool thread for any real, non-precompleted async
    /// I/O (health probe), not guaranteed to be the UI thread that
    /// originally called StartAsync/HealthTickAsync/etc. Any UI-toolkit
    /// subscriber MUST self-marshal (e.g. DispatcherQueue.TryEnqueue) before
    /// touching UI state. Each subscriber is invoked in isolation (see Raise):
    /// one subscriber's exception is caught and logged, never propagated back
    /// into ApplyAsync's own effect chain, and never prevents delivery to the
    /// remaining subscribers.
    /// </summary>
    public event Action<DaemonSnapshot>? SnapshotChanged;

    public async Task StartAsync(CancellationToken ct = default)
    {
        await ApplyAsync(DaemonEvent.Start, ct).ConfigureAwait(false);
    }

    public async Task HealthTickAsync(CancellationToken ct = default)
    {
        await ApplyAsync(DaemonEvent.HealthTick, ct).ConfigureAwait(false);
    }

    private async Task ApplyAsync(DaemonEvent ev, CancellationToken ct)
    {
        DaemonSnapshot next;
        DaemonEffect effect;
        lock (_gate)
        {
            (next, effect) = DaemonSupervisorMachine.Reduce(_snapshot, ev);
            _snapshot = next;
        }
        Raise(next);

        switch (effect)
        {
            case DaemonEffect.Probe:
                await RunProbeAsync(ct).ConfigureAwait(false);
                break;
            case DaemonEffect.ResubscribeSurfaces:
                if (_resubscriber is not null)
                    await _resubscriber.ResubscribeAllAsync(ct).ConfigureAwait(false);
                break;
        }
    }

    private async Task RunProbeAsync(CancellationToken ct)
    {
        bool ok;
        try
        {
            ok = await _probe.ProbeAsync(ct).ConfigureAwait(false);
        }
        catch
        {
            ok = false;
        }
        await ApplyAsync(ok ? DaemonEvent.ProbeSucceeded : DaemonEvent.ProbeFailed, ct)
            .ConfigureAwait(false);
    }

    /// <summary>
    /// Invokes each SnapshotChanged subscriber in isolation: a throwing
    /// subscriber is caught and logged, and can neither stop delivery to the
    /// other subscribers nor unwind back into ApplyAsync's own effect chain
    /// (which would otherwise abort StartAsync/HealthTickAsync/etc. one
    /// statement early — see debug-run-shell-zero-sessions round 2).
    /// </summary>
    private void Raise(DaemonSnapshot snap)
    {
        var handlers = SnapshotChanged;
        if (handlers is null)
            return;

        foreach (var handler in handlers.GetInvocationList())
        {
            try
            {
                ((Action<DaemonSnapshot>)handler)(snap);
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine(
                    $"DaemonSupervisorService.SnapshotChanged subscriber threw: {ex}");
            }
        }
    }
}
