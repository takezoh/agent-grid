using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class IslandStateMachineTests
{
    private static IslandState Compact(int pending = 0) =>
        new(IslandMode.Compact, IslandExpandReason.None, pending);

    [Fact]
    public void Initial_state_is_expanded_without_auto_reason()
    {
        Assert.Equal(IslandMode.Expanded, IslandState.Initial.Mode);
        Assert.Equal(IslandExpandReason.None, IslandState.Initial.Reason);
    }

    [Fact]
    public void Pending_arrival_auto_expands_a_compact_island()
    {
        var s = IslandStateMachine.OnSnapshot(Compact(), pendingCount: 1);
        Assert.Equal(IslandMode.Expanded, s.Mode);
        Assert.Equal(IslandExpandReason.AutoPending, s.Reason);
    }

    [Fact]
    public void New_pending_reexpands_even_when_collapsed_mid_queue()
    {
        // User collapsed while 2 items were pending; a 3rd arrival pops it open.
        var s = IslandStateMachine.OnSnapshot(Compact(pending: 2), pendingCount: 3);
        Assert.Equal(IslandMode.Expanded, s.Mode);
        Assert.Equal(IslandExpandReason.AutoPending, s.Reason);
    }

    [Fact]
    public void Pending_decrease_does_not_reexpand_a_compact_island()
    {
        var s = IslandStateMachine.OnSnapshot(Compact(pending: 3), pendingCount: 2);
        Assert.Equal(IslandMode.Compact, s.Mode);
        Assert.Equal(2, s.PendingCount);
    }

    [Fact]
    public void Auto_expanded_island_collapses_when_queue_drains()
    {
        var expanded = IslandStateMachine.OnSnapshot(Compact(), pendingCount: 2);
        var s = IslandStateMachine.OnSnapshot(expanded, pendingCount: 0);
        Assert.Equal(IslandMode.Compact, s.Mode);
        Assert.Equal(IslandExpandReason.None, s.Reason);
    }

    [Fact]
    public void User_expanded_island_stays_open_when_queue_drains()
    {
        var s = IslandStateMachine.OnUserExpand(Compact(pending: 0));
        s = IslandStateMachine.OnSnapshot(s, pendingCount: 1);
        s = IslandStateMachine.OnSnapshot(s, pendingCount: 0);
        Assert.Equal(IslandMode.Expanded, s.Mode);
        Assert.Equal(IslandExpandReason.User, s.Reason);
    }

    [Fact]
    public void Initial_expanded_state_survives_empty_snapshots()
    {
        // Launch with nothing pending must not immediately collapse the
        // first-run panel (the UI e2e contract reads the expanded tree).
        var s = IslandStateMachine.OnSnapshot(IslandState.Initial, pendingCount: 0);
        Assert.Equal(IslandMode.Expanded, s.Mode);
    }

    [Fact]
    public void User_toggle_flips_mode_and_collapse_clears_reason()
    {
        var s = IslandStateMachine.OnUserToggle(IslandState.Initial);
        Assert.Equal(IslandMode.Compact, s.Mode);
        s = IslandStateMachine.OnUserToggle(s);
        Assert.Equal(IslandMode.Expanded, s.Mode);
        Assert.Equal(IslandExpandReason.User, s.Reason);
        s = IslandStateMachine.OnUserCollapse(s);
        Assert.Equal(IslandExpandReason.None, s.Reason);
    }
}
