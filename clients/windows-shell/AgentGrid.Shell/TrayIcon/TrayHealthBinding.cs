using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.Health;

namespace AgentGrid.Shell.TrayIcon;

/// <summary>
/// Binds DaemonSupervisor snapshots to tray appearance.
/// No dependency edge toward ToastNotifier (structural separation).
/// </summary>
public sealed class TrayHealthBinding
{
    private readonly DaemonSupervisorService _supervisor;
    private TrayAppearance _current = TrayAppearanceMapper.From(DaemonSnapshot.Initial);

    public TrayHealthBinding(DaemonSupervisorService supervisor)
    {
        _supervisor = supervisor;
        _supervisor.SnapshotChanged += OnSnapshot;
        _current = TrayAppearanceMapper.From(_supervisor.Snapshot);
    }

    public TrayAppearance Current => _current;

    public event Action<TrayAppearance>? AppearanceChanged;

    private void OnSnapshot(DaemonSnapshot snap)
    {
        _current = TrayAppearanceMapper.From(snap);
        AppearanceChanged?.Invoke(_current);
    }
}
