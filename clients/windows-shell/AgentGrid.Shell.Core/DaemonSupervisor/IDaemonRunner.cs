namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// I/O shell around the pure state machine. Fakeable for xUnit; real impl spawns wsl.exe.
/// Mirrors platform/agentlaunch Dispatcher seam and fakeagents convention.
/// </summary>
public interface IDaemonRunner
{
    /// <summary>Spawn the WSL-hosted server (or remote-adopt no-op when spawn disabled).</summary>
    Task<SpawnResult> SpawnAsync(CancellationToken ct = default);

    /// <summary>Request graceful shutdown of the daemon.</summary>
    Task ShutdownAsync(CancellationToken ct = default);
}

public enum SpawnResultKind
{
    Started,
    Failed,
    ExitedImmediately,
}

public sealed record SpawnResult(SpawnResultKind Kind, string? Detail = null);

/// <summary>
/// Health probe against the configured gateway (authenticated /api/sessions).
/// Token is read fresh per attempt (contract-b2-token-acquisition).
/// </summary>
public interface IDaemonHealthProbe
{
    Task<bool> ProbeAsync(CancellationToken ct = default);
}

/// <summary>
/// Fan-out resubscribe after daemon restart (contract-b3-restart-continuity).
/// </summary>
public interface ISurfaceResubscriber
{
    Task ResubscribeAllAsync(CancellationToken ct = default);
}
