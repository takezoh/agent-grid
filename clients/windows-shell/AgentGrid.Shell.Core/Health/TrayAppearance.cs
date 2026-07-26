using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Health;

/// <summary>
/// Pure mapping from daemon health state → tray-icon appearance.
/// Driven solely by DaemonSupervisor; MUST NOT depend on ToastNotifier
/// (contract-daemon-health-toast-budget, contract-health-toast-structural-separation).
/// </summary>
public enum TrayIconKind
{
    Connected,
    Connecting,
    Degraded,
    Stopped,
}

public sealed record TrayAppearance(
    TrayIconKind Kind,
    string Tooltip,
    bool IsConnected);

public static class TrayAppearanceMapper
{
    public static TrayAppearance From(DaemonSnapshot snap) =>
        snap.State switch
        {
            DaemonState.Healthy =>
                new TrayAppearance(TrayIconKind.Connected, "Agent Grid — connected", true),

            DaemonState.NotRunning =>
                new TrayAppearance(TrayIconKind.Connecting, "Agent Grid — connecting…", false),

            DaemonState.Degraded =>
                new TrayAppearance(
                    TrayIconKind.Degraded,
                    $"Agent Grid — degraded: {snap.LastFailureReason ?? "unknown"}",
                    false),

            _ =>
                new TrayAppearance(TrayIconKind.Degraded, "Agent Grid — unavailable", false),
        };
}
