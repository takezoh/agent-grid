using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Platform.Toast;

/// <summary>
/// Boundary for OS toast surface. Implementation lives in WinUI
/// (AppNotificationManager); tests use a recording fake.
/// Health sources MUST never call this (contract-daemon-health-toast-budget).
/// </summary>
public interface IToastNotifier
{
    /// <summary>Show approve/deny buttons for an approval item.</summary>
    Task ShowApprovalAsync(ApprovalItem item, CancellationToken ct = default);

    /// <summary>Show question toast (inline text when IME works; else expand-panel action).</summary>
    Task ShowQuestionAsync(QuestionItem item, CancellationToken ct = default);

    /// <summary>Remove a toast when the item is resolved elsewhere.</summary>
    Task DismissAsync(string itemId, CancellationToken ct = default);
}

/// <summary>
/// Routes supervision snapshot deltas into toasts when panel is unwatched.
/// Pure decision via ToastDecisionService; never used for daemon health.
/// </summary>
public sealed class SupervisionToastRouter
{
    private readonly ToastDecisionService _decision;
    private readonly IToastNotifier _notifier;
    private readonly Func<bool> _panelFlyoutOpen;
    private readonly Func<nint> _panelHwnd;
    private readonly HashSet<string> _seen = new(StringComparer.Ordinal);

    public SupervisionToastRouter(
        ToastDecisionService decision,
        IToastNotifier notifier,
        Func<bool> panelFlyoutOpen,
        Func<nint> panelHwnd)
    {
        _decision = decision;
        _notifier = notifier;
        _panelFlyoutOpen = panelFlyoutOpen;
        _panelHwnd = panelHwnd;
    }

    public async Task OnSnapshotAsync(SupervisionSnapshot snap, CancellationToken ct = default)
    {
        var live = new HashSet<string>(StringComparer.Ordinal);
        foreach (var a in snap.Approvals)
        {
            live.Add(a.ApprovalId);
            if (!_seen.Add(a.ApprovalId))
                continue;
            var item = new PanelGlanceItem(a.ApprovalId, a.SessionId, "approval", a.Summary, a.ExpiresAt);
            if (_decision.DecideForNewPending(item, _panelFlyoutOpen(), _panelHwnd()) ==
                ToastDecisionService.ToastOutcome.Fire)
            {
                await _notifier.ShowApprovalAsync(a, ct).ConfigureAwait(false);
            }
        }

        foreach (var q in snap.Questions)
        {
            live.Add(q.QuestionId);
            if (!_seen.Add(q.QuestionId))
                continue;
            var item = new PanelGlanceItem(q.QuestionId, q.SessionId, "question", q.Prompt, q.ExpiresAt);
            if (_decision.DecideForNewPending(item, _panelFlyoutOpen(), _panelHwnd()) ==
                ToastDecisionService.ToastOutcome.Fire)
            {
                await _notifier.ShowQuestionAsync(q, ct).ConfigureAwait(false);
            }
        }

        // Drop ids that left the queue.
        _seen.RemoveWhere(id => !live.Contains(id));
    }
}
