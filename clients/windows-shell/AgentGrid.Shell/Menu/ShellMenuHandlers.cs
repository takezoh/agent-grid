using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Menu;

/// <summary>
/// Structurally separate Quit vs Stop-daemon handlers (FR-B3-05, contract-quit-vs-daemon-stop).
/// Quit MUST NEVER invoke DaemonSupervisor.StopDaemonAsync.
/// </summary>
public sealed class ShellMenuHandlers
{
    private readonly DaemonSupervisorService _supervisor;
    private readonly Action _quitApplication;

    public ShellMenuHandlers(DaemonSupervisorService supervisor, Action quitApplication)
    {
        _supervisor = supervisor ?? throw new ArgumentNullException(nameof(supervisor));
        _quitApplication = quitApplication ?? throw new ArgumentNullException(nameof(quitApplication));
    }

    /// <summary>
    /// Tray Quit: leave the daemon running. Does not call StopDaemonAsync.
    /// </summary>
    public void OnQuit()
    {
        // Intentionally does not touch the daemon.
        _quitApplication();
    }

    /// <summary>
    /// Distinct menu item: stop the daemon then (optionally) continue shell life or quit.
    /// </summary>
    public Task OnStopDaemonAsync(CancellationToken ct = default) =>
        _supervisor.StopDaemonAsync(ct);

    public Task OnRestartDaemonAsync(CancellationToken ct = default) =>
        _supervisor.RestartAsync(ct);
}
