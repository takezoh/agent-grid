using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Panel;

/// <summary>
/// UI-toolkit-agnostic presenter for the always-visible bar + flyout.
/// WinUI/WPF bind to <see cref="Current"/> and subscribe to <see cref="Changed"/>.
/// </summary>
public sealed class PanelGlancePresenter
{
    private readonly ApprovalSubmissionService _supervision;
    private PanelGlanceView _current;

    public PanelGlancePresenter(ApprovalSubmissionService supervision)
    {
        _supervision = supervision;
        _current = PanelGlanceView.From(supervision.Snapshot);
        _supervision.SnapshotChanged += OnSnapshot;
    }

    public PanelGlanceView Current => _current;

    public event Action<PanelGlanceView>? Changed;

    public void Dispose()
    {
        _supervision.SnapshotChanged -= OnSnapshot;
    }

    private void OnSnapshot(SupervisionSnapshot snap)
    {
        _current = PanelGlanceView.From(snap);
        Changed?.Invoke(_current);
    }
}
