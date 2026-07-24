using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.Composition;

public sealed class ShellFleetTests
{
    [Fact]
    public void Aggregate_preserves_same_session_id_on_different_servers()
    {
        var input = new Dictionary<string, SupervisionSnapshot>
        {
            ["one"] = Snapshot("same"),
            ["two"] = Snapshot("same"),
        };

        var aggregate = ShellFleet.AggregateSnapshots(input);

        Assert.Equal(2, aggregate.Sessions.Count);
        Assert.Contains(aggregate.Sessions, session =>
            session.ServerId == "one" && session.SessionId == "same");
        Assert.Contains(aggregate.Sessions, session =>
            session.ServerId == "two" && session.SessionId == "same");
    }

    [Fact]
    public void Aggregate_tags_pending_items_with_their_connection()
    {
        var input = new Dictionary<string, SupervisionSnapshot>
        {
            ["remote"] = new(
                [],
                [new ApprovalItem("a1", "s1", "approve")],
                [],
                [],
                false,
                null),
        };

        var aggregate = ShellFleet.AggregateSnapshots(input);

        Assert.Equal("remote", Assert.Single(aggregate.Approvals).ServerId);
    }

    private static SupervisionSnapshot Snapshot(string sessionId) => new(
        [new SessionSummary(sessionId, SessionPhase.Running)],
        [],
        [],
        [],
        false,
        null);
}
