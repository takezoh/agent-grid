namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Health probe against the configured gateway (authenticated /api/sessions).
/// Token is read fresh per attempt.
/// </summary>
public interface IDaemonHealthProbe
{
    Task<bool> ProbeAsync(CancellationToken ct = default);
}

/// <summary>
/// Reconnects client surfaces after gateway availability is restored.
/// </summary>
public interface ISurfaceResubscriber
{
    Task ResubscribeAllAsync(CancellationToken ct = default);
}
