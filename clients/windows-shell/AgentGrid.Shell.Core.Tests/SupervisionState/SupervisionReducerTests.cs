using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class SupervisionReducerTests
{
    [Fact]
    public void Approval_requested_adds_to_queue()
    {
        var s = SupervisionSnapshot.Empty;
        s = SupervisionReducer.Reduce(s, new EvtApprovalRequested("a1", "s1", "run rm -rf"));
        Assert.Single(s.Approvals);
        Assert.Equal("a1", s.Approvals[0].ApprovalId);
    }

    [Fact]
    public void Optimistic_submit_removes_item()
    {
        var s = WithApproval("a1", "s1", "cmd");
        s = SupervisionReducer.Reduce(s, new EvtApprovalSubmitRequested("a1", "s1", "accept"));
        Assert.Empty(s.Approvals);
    }

    [Fact]
    public void Submit_failed_rolls_back()
    {
        var s = WithApproval("a1", "s1", "cmd");
        s = SupervisionReducer.Reduce(s, new EvtApprovalSubmitRequested("a1", "s1", "accept"));
        Assert.Empty(s.Approvals);

        s = SupervisionReducer.Reduce(s, new EvtApprovalSubmitFailed(
            "a1", "s1", "cmd", ExpiresAt: null, Reason: "network"));
        Assert.Single(s.Approvals);
        Assert.Equal("a1", s.Approvals[0].ApprovalId);
        Assert.Equal("cmd", s.Approvals[0].Summary);
    }

    [Fact]
    public void Resolved_by_other_shows_notice_and_removes_item()
    {
        var s = WithApproval("a1", "s1", "cmd");
        s = SupervisionReducer.Reduce(s, new EvtApprovalSubmitRequested("a1", "s1", "accept"));
        s = SupervisionReducer.Reduce(s, new EvtApprovalResolvedByOther("a1", "s1"));

        Assert.Empty(s.Approvals);
        Assert.Single(s.AlreadyHandled);
        Assert.Equal("a1", s.AlreadyHandled[0].ItemId);
        Assert.Contains("another client", s.AlreadyHandled[0].Message, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public void Connection_failed_surfaces_explicit_failure()
    {
        var s = SupervisionReducer.Reduce(
            SupervisionSnapshot.Empty,
            new EvtConnectionFailed("token unreadable"));
        Assert.True(s.ConnectionFailed);
        Assert.Equal("token unreadable", s.ConnectionFailureReason);
    }

    [Fact]
    public void View_update_replaces_sessions()
    {
        var s = SupervisionReducer.Reduce(
            SupervisionSnapshot.Empty,
            new EvtViewUpdateSessions(new[]
            {
                new SessionSummary("s1", SessionPhase.Running),
                new SessionSummary("s2", SessionPhase.Waiting),
            }));
        Assert.Equal(2, s.Sessions.Count);
    }

    [Fact]
    public void Question_request_and_resolve()
    {
        var s = SupervisionSnapshot.Empty;
        s = SupervisionReducer.Reduce(s, new EvtQuestionRequested("q1", "s1", "which branch?"));
        Assert.Single(s.Questions);
        s = SupervisionReducer.Reduce(s, new EvtQuestionResolved("q1", "s1"));
        Assert.Empty(s.Questions);
    }

    // witness-frame-flood-adversarial: same-id ar is upserted, never stacked.
    [Fact]
    public void Duplicate_approval_request_keeps_a_single_card()
    {
        var s = WithApproval("a1", "s1", "cmd-v1");
        s = SupervisionReducer.Reduce(s, new EvtApprovalRequested("a1", "s1", "cmd-v2"));
        Assert.Single(s.Approvals);
        Assert.Equal("a1", s.Approvals[0].ApprovalId);
        Assert.Equal("cmd-v2", s.Approvals[0].Summary);
    }

    // witness-quiet-normal: ax resolves pending without inventing a notice or ghost card.
    [Fact]
    public void Approval_resolved_removes_item_without_notice()
    {
        var s = WithApproval("a1", "s1", "cmd");
        s = SupervisionReducer.Reduce(s, new EvtApprovalResolved("a1", "s1", "accept", "ci-A"));
        Assert.Empty(s.Approvals);
        Assert.Empty(s.AlreadyHandled);
        Assert.Equal(0, s.PendingCount);
    }

    // witness-lifecycle-adversarial: notice upsert stays high-water 1 for same id.
    [Fact]
    public void Duplicate_resolved_by_other_keeps_a_single_notice()
    {
        var s = WithApproval("a1", "s1", "cmd");
        s = SupervisionReducer.Reduce(s, new EvtApprovalSubmitRequested("a1", "s1", "accept"));
        s = SupervisionReducer.Reduce(s, new EvtApprovalResolvedByOther("a1", "s1"));
        s = SupervisionReducer.Reduce(s, new EvtApprovalResolvedByOther("a1", "s1"));
        Assert.Empty(s.Approvals);
        Assert.Single(s.AlreadyHandled);
    }

    private static SupervisionSnapshot WithApproval(string id, string session, string summary) =>
        SupervisionReducer.Reduce(
            SupervisionSnapshot.Empty,
            new EvtApprovalRequested(id, session, summary));
}
