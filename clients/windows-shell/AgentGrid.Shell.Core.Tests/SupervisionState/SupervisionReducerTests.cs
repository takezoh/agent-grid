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

    private static SupervisionSnapshot WithApproval(string id, string session, string summary) =>
        SupervisionReducer.Reduce(
            SupervisionSnapshot.Empty,
            new EvtApprovalRequested(id, session, summary));
}
