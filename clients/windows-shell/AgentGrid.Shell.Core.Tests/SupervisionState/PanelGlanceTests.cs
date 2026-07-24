using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class PanelGlanceTests
{
    [Fact]
    public void Projects_pending_approvals_and_questions()
    {
        var snap = SupervisionSnapshot.Empty with
        {
            Approvals = new[]
            {
                new ApprovalItem("ap1", "s1", "echo hi"),
            },
            Questions = new[]
            {
                new QuestionItem("q1", "s1", "Continue?"),
            },
            Sessions = new[]
            {
                new SessionSummary("s1", SessionPhase.Waiting, "T"),
            },
        };

        var view = PanelGlanceView.From(snap);
        Assert.Equal(2, view.PendingCount);
        Assert.Contains(view.Pending, i => i.Kind == "approval" && i.Headline == "echo hi");
        Assert.Contains(view.Pending, i => i.Kind == "question");
        Assert.Contains("pending", view.StatusLine, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public void Connection_failed_status_line()
    {
        var snap = SupervisionSnapshot.Empty with
        {
            ConnectionFailed = true,
            ConnectionFailureReason = "ws down",
        };
        var view = PanelGlanceView.From(snap);
        Assert.Contains("Disconnected", view.StatusLine, StringComparison.Ordinal);
        Assert.Contains("ws down", view.StatusLine, StringComparison.Ordinal);
    }
}
