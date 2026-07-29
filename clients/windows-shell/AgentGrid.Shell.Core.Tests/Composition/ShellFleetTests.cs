using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.Health;
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

    [Fact]
    public void Default_workspace_command_resolves_to_the_local_install()
    {
        var path = ShellFleet.ResolveWorkspaceExecutable(
            "agent-grid-workspace",
            @"C:\Users\test\AppData\Local");

        // Production uses Path.Combine, so separators follow the host OS.
        // Assert the Windows install layout and executable name, not separator style.
        Assert.Equal(
            NormalizeSeparators(@"C:\Users\test\AppData\Local\agent-grid\workspace\agent-grid-workspace.exe"),
            NormalizeSeparators(path));
        Assert.Equal(
            @"D:\custom\workspace.exe",
            ShellFleet.ResolveWorkspaceExecutable(
                @"D:\custom\workspace.exe",
                @"C:\Users\test\AppData\Local"));
    }

    private static string NormalizeSeparators(string path) =>
        path.Replace('\\', Path.DirectorySeparatorChar)
            .Replace('/', Path.DirectorySeparatorChar);

    [Fact]
    public void Shell_launches_workspace_without_a_standalone_home_window()
    {
        Assert.Equal(
            ["--config-dir", @"C:\test-config", "--background"],
            ShellFleet.WorkspaceLaunchArguments(@"C:\test-config"));
    }

    [Fact]
    public void Tray_aggregate_preserves_protocol_mismatch_cause_and_healthy_peer()
    {
        var connected = new TrayAppearance(
            TrayIconKind.Connected,
            "Agent Grid — connected",
            true);
        var degraded = new TrayAppearance(
            TrayIconKind.Degraded,
            "Agent Grid — degraded",
            false);

        var aggregate = ShellFleet.AggregateTrayAppearances([
            (connected, null),
            (degraded, "incompatible protocol: client protocol-2, server protocol-1"),
        ]);

        Assert.Equal(TrayIconKind.Degraded, aggregate.Kind);
        Assert.False(aggregate.IsConnected);
        Assert.Contains("1/2 servers incompatible", aggregate.Tooltip);
        Assert.Contains("client protocol-2, server protocol-1", aggregate.Tooltip);
    }

    private static SupervisionSnapshot Snapshot(string sessionId) => new(
        [new SessionSummary(sessionId, SessionPhase.Running)],
        [],
        [],
        [],
        false,
        null);
}
