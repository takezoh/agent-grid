using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Platform.Toast;

/// <summary>
/// Decides whether to fire a toast for a supervision event.
/// Combines panel-watched predicate + health-toast budget
/// (contract-toast-panel-watched-detection, contract-daemon-health-toast-budget).
/// </summary>
public sealed class ToastDecisionService
{
    private readonly PanelWatchedPredicate _watched;

    public ToastDecisionService(PanelWatchedPredicate watched) =>
        _watched = watched ?? throw new ArgumentNullException(nameof(watched));

    public enum ToastOutcome
    {
        Fire,
        SuppressWatched,
        SuppressBudget,
        SuppressSource,
    }

    public ToastOutcome Decide(
        string source,
        bool panelFlyoutOpen,
        nint panelHwnd)
    {
        if (!HealthToastBudget.IsSupervisionToastSource(source))
            return ToastOutcome.SuppressSource;

        if (!_watched.ShouldFireToast(panelFlyoutOpen, panelHwnd))
            return ToastOutcome.SuppressWatched;

        return ToastOutcome.Fire;
    }

    /// <summary>
    /// Convenience: fire iff a new pending approval/question arrived and panel unwatched.
    /// </summary>
    public ToastOutcome DecideForNewPending(
        PanelGlanceItem item,
        bool panelFlyoutOpen,
        nint panelHwnd) =>
        Decide(item.Kind, panelFlyoutOpen, panelHwnd);
}
