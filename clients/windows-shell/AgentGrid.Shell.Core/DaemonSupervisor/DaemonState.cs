namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Pure state labels for client-side connection health.
/// </summary>
public enum DaemonState
{
    NotRunning,
    Healthy,
    Degraded,
}

/// <summary>
/// Events that drive the pure DaemonSupervisor state machine.
/// </summary>
public enum DaemonEvent
{
    /// <summary>Shell boot or explicit Start requested.</summary>
    Start,

    /// <summary>Authenticated /api/sessions probe succeeded.</summary>
    ProbeSucceeded,

    /// <summary>Authenticated /api/sessions probe failed (or token unreadable).</summary>
    ProbeFailed,

    /// <summary>Periodic health tick while connected or degraded.</summary>
    HealthTick,
}

/// <summary>
/// Side-effect intents produced by the pure reducer. The Runner I/O shell executes them.
/// </summary>
public enum DaemonEffect
{
    None,
    Probe,
    ResubscribeSurfaces,
}

/// <summary>
/// Immutable snapshot of the connection-health machine.
/// </summary>
public sealed record DaemonSnapshot(
    DaemonState State,
    int ProbeFailures,
    string? LastFailureReason)
{
    public bool IsConnected => State is DaemonState.Healthy;

    public static DaemonSnapshot Initial { get; } = new(
        DaemonState.NotRunning,
        ProbeFailures: 0,
        LastFailureReason: null);
}
