namespace AgentGrid.Shell.Core.SupervisionState;

public enum SessionPhase
{
    Running,
    Waiting,
    Failed,
    Done,
}

public sealed record SessionSummary(
    string SessionId,
    SessionPhase Phase,
    string? Title = null);

public sealed record ApprovalItem(
    string ApprovalId,
    string SessionId,
    string Summary,
    DateTimeOffset? ExpiresAt = null);

public sealed record QuestionItem(
    string QuestionId,
    string SessionId,
    string Prompt,
    DateTimeOffset? ExpiresAt = null);

/// <summary>
/// Explicit already-handled outcome when another client won the race
/// (contract-resolved-by-other-display, FR-APPROVE-RESOLVED-BY-OTHER).
/// </summary>
public sealed record AlreadyHandledNotice(
    string ItemId,
    string SessionId,
    string Kind, // "approval" | "question"
    string Message);

public sealed record SupervisionSnapshot(
    IReadOnlyList<SessionSummary> Sessions,
    IReadOnlyList<ApprovalItem> Approvals,
    IReadOnlyList<QuestionItem> Questions,
    IReadOnlyList<AlreadyHandledNotice> AlreadyHandled,
    bool ConnectionFailed,
    string? ConnectionFailureReason)
{
    public static SupervisionSnapshot Empty { get; } = new(
        Array.Empty<SessionSummary>(),
        Array.Empty<ApprovalItem>(),
        Array.Empty<QuestionItem>(),
        Array.Empty<AlreadyHandledNotice>(),
        ConnectionFailed: false,
        ConnectionFailureReason: null);

    public int PendingCount => Approvals.Count + Questions.Count;
}

/// <summary>Inbound events reduced by SupervisionReducer.</summary>
public abstract record SupervisionEvent;

public sealed record EvtApprovalRequested(
    string ApprovalId,
    string SessionId,
    string Summary,
    DateTimeOffset? ExpiresAt = null) : SupervisionEvent;

public sealed record EvtApprovalResolved(
    string ApprovalId,
    string SessionId,
    string Decision,
    string? DecidedBy = null) : SupervisionEvent;

public sealed record EvtQuestionRequested(
    string QuestionId,
    string SessionId,
    string Prompt,
    DateTimeOffset? ExpiresAt = null) : SupervisionEvent;

public sealed record EvtQuestionResolved(
    string QuestionId,
    string SessionId,
    string? DecidedBy = null) : SupervisionEvent;

public sealed record EvtViewUpdateSessions(
    IReadOnlyList<SessionSummary> Sessions) : SupervisionEvent;

public sealed record EvtConnectionFailed(string Reason) : SupervisionEvent;

public sealed record EvtConnectionRestored : SupervisionEvent;

/// <summary>
/// User submitted approve/deny. Reducer optimistically removes the item;
/// outcome events (SubmitSucceeded / SubmitFailed / ResolvedByOther) finalize.
/// </summary>
public sealed record EvtApprovalSubmitRequested(
    string ApprovalId,
    string SessionId,
    string Decision) : SupervisionEvent;

public sealed record EvtApprovalSubmitSucceeded(string ApprovalId) : SupervisionEvent;

public sealed record EvtApprovalSubmitFailed(
    string ApprovalId,
    string SessionId,
    string Summary,
    DateTimeOffset? ExpiresAt,
    string Reason) : SupervisionEvent;

public sealed record EvtApprovalResolvedByOther(
    string ApprovalId,
    string SessionId) : SupervisionEvent;
