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

    public event Action<SupervisionSnapshot>? SnapshotChanged;

    public void Apply(SupervisionEvent ev)
    {
        SupervisionSnapshot next;
        lock (_gate)
        {
            next = SupervisionReducer.Reduce(_state, ev);
            _state = next;
        }
        SnapshotChanged?.Invoke(next);
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
}
