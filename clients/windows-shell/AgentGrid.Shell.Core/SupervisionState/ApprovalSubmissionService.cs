using AgentGrid.Shell.Core.GatewayClient;

namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>
/// Orchestrates optimistic approve/deny + rollback / resolved-by-other
/// (contract-approve-submission-rollback, contract-resolved-by-other-display).
/// </summary>
public sealed class ApprovalSubmissionService
{
    private readonly ShellGatewayClient _gateway;
    private SupervisionSnapshot _state;
    private readonly object _gate = new();

    public ApprovalSubmissionService(ShellGatewayClient gateway, SupervisionSnapshot? initial = null)
    {
        _gateway = gateway;
        _state = initial ?? SupervisionSnapshot.Empty;
    }

    public SupervisionSnapshot Snapshot
    {
        get { lock (_gate) return _state; }
    }

    /// <summary>
    /// Fires on whatever thread calls Apply — for the production wiring, that
    /// is the WS receive-loop's thread-pool thread (SupervisionWsSession), not
    /// guaranteed to be the UI thread. Any UI-toolkit subscriber MUST
    /// self-marshal (e.g. DispatcherQueue.TryEnqueue) before touching UI
    /// state. Each subscriber is invoked in isolation (see Raise): one
    /// subscriber's exception is caught and logged, never propagated back
    /// into Apply's caller, and never prevents delivery to the remaining
    /// subscribers.
    /// </summary>
    public event Action<SupervisionSnapshot>? SnapshotChanged;

    public void Apply(SupervisionEvent ev)
    {
        SupervisionSnapshot next;
        lock (_gate)
        {
            next = SupervisionReducer.Reduce(_state, ev);
            _state = next;
        }
        Raise(next);
    }

    /// <summary>
    /// Invokes each SnapshotChanged subscriber in isolation: a throwing
    /// subscriber is caught and logged, and can neither stop delivery to the
    /// other subscribers nor unwind back into Apply's caller (see
    /// debug-run-shell-zero-sessions round 2 — the same defect shape as
    /// DaemonSupervisorService.Raise, on the toast/approval producer).
    /// </summary>
    private void Raise(SupervisionSnapshot next)
    {
        var handlers = SnapshotChanged;
        if (handlers is null)
            return;

        foreach (var handler in handlers.GetInvocationList())
        {
            try
            {
                ((Action<SupervisionSnapshot>)handler)(next);
            }
            catch (Exception ex)
            {
                System.Diagnostics.Debug.WriteLine(
                    $"ApprovalSubmissionService.SnapshotChanged subscriber threw: {ex}");
            }
        }
    }

    public async Task SubmitApprovalAsync(
        string approvalId,
        string sessionId,
        string decision,
        string summary,
        DateTimeOffset? expiresAt = null,
        CancellationToken ct = default)
    {
        // Optimistic removal first.
        Apply(new EvtApprovalSubmitRequested(approvalId, sessionId, decision));

        var result = await _gateway.RespondApprovalAsync(sessionId, approvalId, decision, ct)
            .ConfigureAwait(false);

        switch (result)
        {
            case ApprovalSubmitResult.Accepted:
                Apply(new EvtApprovalSubmitSucceeded(approvalId));
                break;
            case ApprovalSubmitResult.ResolvedByOther:
                Apply(new EvtApprovalResolvedByOther(approvalId, sessionId));
                break;
            default:
                // Network/server error → rollback
                Apply(new EvtApprovalSubmitFailed(approvalId, sessionId, summary, expiresAt, result.ToString()));
                break;
        }
    }

    /// <summary>
    /// Submit free-text question answer. On success removes the item; on conflict
    /// surfaces already-handled; on network error leaves the item in place.
    /// </summary>
    public async Task SubmitQuestionAsync(
        string questionId,
        string sessionId,
        string answer,
        string prompt,
        CancellationToken ct = default)
    {
        // Optimistic removal (reuse approval submit event shape via local resolved).
        Apply(new EvtQuestionResolved(questionId, sessionId, "local-optimistic"));

        var result = await _gateway.RespondQuestionAsync(sessionId, questionId, answer, ct)
            .ConfigureAwait(false);

        switch (result)
        {
            case ApprovalSubmitResult.Accepted:
                // Already removed.
                break;
            case ApprovalSubmitResult.ResolvedByOther:
                Apply(new EvtApprovalResolvedByOther(questionId, sessionId));
                // Re-shape notice kind via a question re-insert is unnecessary;
                // AlreadyHandled message is enough for glance.
                break;
            default:
                // Rollback: put question back.
                Apply(new EvtQuestionRequested(questionId, sessionId, prompt));
                break;
        }
    }
}
