namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>
/// Pure reducer over boundary-2 WS events → panel/tray glance state.
/// Optimistic submission + rollback (contract-approve-submission-rollback).
/// Authoritative resolved-by-other without client-side dedupe flags
/// (contract-resolved-by-other-display).
/// </summary>
public static class SupervisionReducer
{
    public static SupervisionSnapshot Reduce(SupervisionSnapshot state, SupervisionEvent ev) =>
        ev switch
        {
            EvtViewUpdateSessions e =>
                state with { Sessions = e.Sessions.ToList() },

            EvtApprovalRequested e =>
                state with
                {
                    Approvals = Upsert(
                        state.Approvals,
                        a => a.ApprovalId == e.ApprovalId,
                        new ApprovalItem(e.ApprovalId, e.SessionId, e.Summary, e.ExpiresAt)),
                    // Clear any prior already-handled notice for the same id (re-issue).
                    AlreadyHandled = state.AlreadyHandled
                        .Where(n => n.ItemId != e.ApprovalId)
                        .ToList(),
                },

            EvtApprovalResolved e =>
                state with
                {
                    Approvals = state.Approvals.Where(a => a.ApprovalId != e.ApprovalId).ToList(),
                },

            EvtQuestionRequested e =>
                state with
                {
                    Questions = Upsert(
                        state.Questions,
                        q => q.QuestionId == e.QuestionId,
                        new QuestionItem(e.QuestionId, e.SessionId, e.Prompt, e.ExpiresAt)),
                    AlreadyHandled = state.AlreadyHandled
                        .Where(n => n.ItemId != e.QuestionId)
                        .ToList(),
                },

            EvtQuestionResolved e =>
                state with
                {
                    Questions = state.Questions.Where(q => q.QuestionId != e.QuestionId).ToList(),
                },

            // Optimistic removal on submit (common path: near-instant UI).
            EvtApprovalSubmitRequested e =>
                OptimisticRemoveApproval(state, e.ApprovalId),

            EvtApprovalSubmitSucceeded e =>
                // Already removed; ensure absence.
                state with
                {
                    Approvals = state.Approvals.Where(a => a.ApprovalId != e.ApprovalId).ToList(),
                },

            // Rollback on network/server error (UAC-006).
            EvtApprovalSubmitFailed e =>
                state with
                {
                    Approvals = Upsert(
                        state.Approvals,
                        a => a.ApprovalId == e.ApprovalId,
                        new ApprovalItem(e.ApprovalId, e.SessionId, e.Summary, e.ExpiresAt)),
                },

            // Authoritative race loss: explicit already-handled, no duplicate submit (UAC-006r).
            // Used for both approval and question race-loss (ItemId is the pending id).
            EvtApprovalResolvedByOther e =>
                state with
                {
                    Approvals = state.Approvals.Where(a => a.ApprovalId != e.ApprovalId).ToList(),
                    Questions = state.Questions.Where(q => q.QuestionId != e.ApprovalId).ToList(),
                    AlreadyHandled = Upsert(
                        state.AlreadyHandled,
                        n => n.ItemId == e.ApprovalId,
                        new AlreadyHandledNotice(
                            e.ApprovalId,
                            e.SessionId,
                            Kind: "approval",
                            Message: "Already handled by another client")),
                },

            EvtConnectionFailed e =>
                state with
                {
                    ConnectionFailed = true,
                    ConnectionFailureReason = e.Reason,
                },

            EvtConnectionRestored =>
                state with
                {
                    ConnectionFailed = false,
                    ConnectionFailureReason = null,
                },

            _ => state,
        };

    private static SupervisionSnapshot OptimisticRemoveApproval(
        SupervisionSnapshot state,
        string approvalId) =>
        state with
        {
            Approvals = state.Approvals.Where(a => a.ApprovalId != approvalId).ToList(),
        };

    private static IReadOnlyList<T> Upsert<T>(
        IReadOnlyList<T> list,
        Func<T, bool> match,
        T item)
    {
        var copy = list.Where(x => !match(x)).ToList();
        copy.Add(item);
        return copy;
    }
}
