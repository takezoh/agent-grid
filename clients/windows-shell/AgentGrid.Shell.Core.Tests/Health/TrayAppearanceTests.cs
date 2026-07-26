using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.Health;

namespace AgentGrid.Shell.Core.Tests.Health;

public class TrayAppearanceTests
{
    [Theory]
    [InlineData(DaemonState.Healthy, TrayIconKind.Connected, true)]
    [InlineData(DaemonState.Degraded, TrayIconKind.Degraded, false)]
    [InlineData(DaemonState.NotRunning, TrayIconKind.Connecting, false)]
    public void Maps_state_to_appearance(DaemonState state, TrayIconKind kind, bool connected)
    {
        var snap = DaemonSnapshot.Initial with { State = state, LastFailureReason = "x" };
        var a = TrayAppearanceMapper.From(snap);
        Assert.Equal(kind, a.Kind);
        Assert.Equal(connected, a.IsConnected);
    }
}
