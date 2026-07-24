namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Pure state labels for Boundary-3 daemon lifecycle.
/// See adr-20260724-boundary-3-adopt-before-spawn and contract-b3-daemon-supervisor-state-machine.
/// </summary>
public enum DaemonState
{
    NotRunning,
    Spawning,
    Healthy,
    Adopted,
    Degraded,
    Swapping,
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

    /// <summary>Spawn process launch returned success (process started; may still crash).</summary>
    SpawnStarted,

    /// <summary>Spawn failed to launch (wsl.exe missing, path invalid, etc.).</summary>
    SpawnFailed,

    /// <summary>Spawned process exited immediately after spawn.</summary>
    SpawnExitedImmediately,

    /// <summary>User selected Restart daemon.</summary>
    RestartRequested,

    /// <summary>Graceful shutdown of the daemon completed during swap.</summary>
    ShutdownCompleted,

    /// <summary>User selected Stop daemon (structurally distinct from Quit).</summary>
    StopRequested,

    /// <summary>Periodic health tick while Healthy/Adopted/Degraded.</summary>
    HealthTick,
}

/// <summary>
/// Side-effect intents produced by the pure reducer. The Runner I/O shell executes them.
/// </summary>
public enum DaemonEffect
{
    None,
    Probe,
    Spawn,
    Shutdown,
    ResubscribeSurfaces,
}

/// <summary>
/// Immutable snapshot of the daemon supervisor machine.
/// Connected is true only for Healthy or Adopted (FR-B3-01).
/// </summary>
public sealed record DaemonSnapshot(
    DaemonState State,
    int SpawnCountThisBoot,
    int ProbeFailures,
    string? LastFailureReason,
    bool HasSpawnedThisBoot)
{
    public bool IsConnected => State is DaemonState.Healthy or DaemonState.Adopted;

    public static DaemonSnapshot Initial { get; } = new(
        DaemonState.NotRunning,
        SpawnCountThisBoot: 0,
        ProbeFailures: 0,
        LastFailureReason: null,
        HasSpawnedThisBoot: false);
}
