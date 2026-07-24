using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Health;

/// <summary>
/// contract-daemon-health-toast-budget: daemon health changes update tray only.
/// Zero non-supervision toasts. Pure guard used by composition / tests.
/// </summary>
public static class HealthToastBudget
{
    /// <summary>
    /// Returns true if a toast would be a budget violation for this health transition.
    /// Health flap (Healthy↔Degraded) must never emit a toast.
    /// </summary>
    public static bool WouldViolateBudget(DaemonState from, DaemonState to) =>
        from != to; // any pure-health transition → toast would violate

    /// <summary>
    /// Allowed toast sources: only supervision items (approval/question), never health.
    /// </summary>
    public static bool IsSupervisionToastSource(string source) =>
        source is "approval" or "question";
}
