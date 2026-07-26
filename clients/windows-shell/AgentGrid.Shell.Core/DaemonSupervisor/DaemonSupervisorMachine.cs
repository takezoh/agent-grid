namespace AgentGrid.Shell.Core.DaemonSupervisor;

/// <summary>
/// Pure client connection-health state machine.
/// MUST NOT depend on ToastNotifier (contract-health-toast-structural-separation).
/// </summary>
public static class DaemonSupervisorMachine
{
    /// <summary>
    /// Reduce (state, event) → (state', effect).
    /// The client only probes the configured gateway; it never starts or stops it.
    /// </summary>
    public static (DaemonSnapshot Next, DaemonEffect Effect) Reduce(DaemonSnapshot current, DaemonEvent ev)
    {
        return (current.State, ev) switch
        {
            (DaemonState.NotRunning, DaemonEvent.Start) =>
                (current with { LastFailureReason = null }, DaemonEffect.Probe),

            (DaemonState.NotRunning, DaemonEvent.ProbeSucceeded) =>
                (current with
                {
                    State = DaemonState.Healthy,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.ResubscribeSurfaces),

            (DaemonState.NotRunning, DaemonEvent.ProbeFailed) =>
                Degraded(current, "health probe failed"),

            (DaemonState.Healthy, DaemonEvent.ProbeSucceeded) =>
                (current with { ProbeFailures = 0 }, DaemonEffect.None),

            (DaemonState.Healthy, DaemonEvent.HealthTick) =>
                (current, DaemonEffect.Probe),

            (DaemonState.Healthy, DaemonEvent.ProbeFailed) =>
                (current with
                {
                    State = DaemonState.Degraded,
                    ProbeFailures = current.ProbeFailures + 1,
                    LastFailureReason = "health probe failed",
                }, DaemonEffect.None),

            (DaemonState.Degraded, DaemonEvent.HealthTick) =>
                (current, DaemonEffect.Probe),

            (DaemonState.Degraded, DaemonEvent.ProbeSucceeded) =>
                (current with
                {
                    State = DaemonState.Healthy,
                    ProbeFailures = 0,
                    LastFailureReason = null,
                }, DaemonEffect.ResubscribeSurfaces),

            (DaemonState.Degraded, DaemonEvent.ProbeFailed) =>
                (current with { ProbeFailures = current.ProbeFailures + 1 }, DaemonEffect.None),

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
