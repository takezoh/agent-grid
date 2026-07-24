namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Pure NotRunning→Spawning→Healthy↔Degraded→Swapping/Adopted state machine.
/// Sole authoring source for daemon-health state (contract-b3-daemon-supervisor-state-machine).
/// MUST NOT depend on ToastNotifier (contract-health-toast-structural-separation).
/// </summary>
public static class DaemonSupervisorMachine
{
    /// <summary>
    /// Reduce (state, event) → (state', effect).
    /// Adopt-before-spawn; at most one spawn per boot (FR-B3-02, FR-B3-03).
    /// </summary>
    public static (DaemonSnapshot Next, DaemonEffect Effect) Reduce(DaemonSnapshot current, DaemonEvent ev)
    {
        return (current.State, ev) switch
        {
            // --- Start / boot: always adopt-first (probe) ---
            (DaemonState.NotRunning, DaemonEvent.Start) =>
                (current with { LastFailureReason = null }, DaemonEffect.Probe),

            (DaemonState.NotRunning, DaemonEvent.ProbeSucceeded) =>
                (current with
                {
                    State = DaemonState.Adopted,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.None),

            (DaemonState.NotRunning, DaemonEvent.ProbeFailed) when !current.HasSpawnedThisBoot =>
                (current with
                {
                    State = DaemonState.Spawning,
                    ProbeFailures = current.ProbeFailures + 1,
                }, DaemonEffect.Spawn),

            (DaemonState.NotRunning, DaemonEvent.ProbeFailed) =>
                Degraded(current, "probe failed after prior spawn"),

            // --- Spawning ---
            (DaemonState.Spawning, DaemonEvent.SpawnStarted) =>
                (current with
                {
                    SpawnCountThisBoot = current.SpawnCountThisBoot + 1,
                    HasSpawnedThisBoot = true,
                }, DaemonEffect.Probe),

            (DaemonState.Spawning, DaemonEvent.SpawnFailed) =>
                Degraded(current with { HasSpawnedThisBoot = true }, "spawn failed to launch"),

            (DaemonState.Spawning, DaemonEvent.ProbeSucceeded) =>
                // Fresh spawn (cold or post-restart): surfaces must resubscribe WS
                // (contract-b3-restart-continuity).
                (current with
                {
                    State = DaemonState.Healthy,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.ResubscribeSurfaces),

            (DaemonState.Spawning, DaemonEvent.ProbeFailed) =>
                // One post-spawn adopt retry then Degraded (FR-B3-03): no second concurrent spawn.
                Degraded(current, "spawned process not healthy"),

            (DaemonState.Spawning, DaemonEvent.SpawnExitedImmediately) =>
                Degraded(current, "spawned process exited immediately"),

            // --- Healthy / Adopted: health ticks ---
            (DaemonState.Healthy, DaemonEvent.ProbeSucceeded) or
            (DaemonState.Adopted, DaemonEvent.ProbeSucceeded) =>
                (current with { ProbeFailures = 0 }, DaemonEffect.None),

            (DaemonState.Healthy, DaemonEvent.HealthTick) or
            (DaemonState.Adopted, DaemonEvent.HealthTick) =>
                (current, DaemonEffect.Probe),

            (DaemonState.Healthy, DaemonEvent.ProbeFailed) or
            (DaemonState.Adopted, DaemonEvent.ProbeFailed) =>
                (current with
                {
                    State = DaemonState.Degraded,
                    ProbeFailures = current.ProbeFailures + 1,
                    LastFailureReason = "health probe failed",
                }, DaemonEffect.None),

            // --- Degraded: keep probing; never auto-spawn a second instance ---
            (DaemonState.Degraded, DaemonEvent.HealthTick) =>
                (current, DaemonEffect.Probe),

            (DaemonState.Degraded, DaemonEvent.ProbeSucceeded) =>
                (current with
                {
                    State = current.HasSpawnedThisBoot ? DaemonState.Healthy : DaemonState.Adopted,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.None),

            (DaemonState.Degraded, DaemonEvent.ProbeFailed) =>
                (current with { ProbeFailures = current.ProbeFailures + 1 }, DaemonEffect.None),

            // --- Restart (swap): graceful shutdown → re-spawn → resubscribe ---
            (DaemonState.Healthy, DaemonEvent.RestartRequested) or
            (DaemonState.Adopted, DaemonEvent.RestartRequested) or
            (DaemonState.Degraded, DaemonEvent.RestartRequested) =>
                (current with { State = DaemonState.Swapping, LastFailureReason = null }, DaemonEffect.Shutdown),

            (DaemonState.Swapping, DaemonEvent.ShutdownCompleted) =>
                // Restart is an explicit user action; allow a fresh spawn for this cycle.
                (current with
                {
                    State = DaemonState.Spawning,
                    HasSpawnedThisBoot = false,
                    SpawnCountThisBoot = 0,
                }, DaemonEffect.Spawn),

            (DaemonState.Swapping, DaemonEvent.ProbeSucceeded) =>
                (current with
                {
                    State = DaemonState.Healthy,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.ResubscribeSurfaces),

            // --- Stop daemon (explicit only; Quit must never fire this) ---
            (DaemonState.Healthy, DaemonEvent.StopRequested) or
            (DaemonState.Adopted, DaemonEvent.StopRequested) or
            (DaemonState.Degraded, DaemonEvent.StopRequested) or
            (DaemonState.Spawning, DaemonEvent.StopRequested) or
            (DaemonState.Swapping, DaemonEvent.StopRequested) =>
                (current with
                {
                    State = DaemonState.NotRunning,
                    LastFailureReason = null,
                }, DaemonEffect.Shutdown),

            // Default: ignore illegal transitions (stay put, no effect).
            _ => (current, DaemonEffect.None),
        };
    }

    private static (DaemonSnapshot, DaemonEffect) Degraded(DaemonSnapshot current, string reason) =>
        (current with
        {
            State = DaemonState.Degraded,
            LastFailureReason = reason,
            ProbeFailures = current.ProbeFailures + 1,
        }, DaemonEffect.None);
}
