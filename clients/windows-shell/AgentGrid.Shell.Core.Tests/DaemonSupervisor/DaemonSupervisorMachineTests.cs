using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

public sealed class DaemonSupervisorMachineTests
{
    [Fact]
    public void Start_only_probes_the_configured_server()
    {
        var (next, effect) =
            DaemonSupervisorMachine.Reduce(DaemonSnapshot.Initial, DaemonEvent.Start);

        Assert.Equal(DaemonState.NotRunning, next.State);
        Assert.Equal(DaemonEffect.Probe, effect);
    }

    [Fact]
    public void Failed_probe_degrades_without_server_lifecycle_effect()
    {
        var (next, effect) =
            DaemonSupervisorMachine.Reduce(DaemonSnapshot.Initial, DaemonEvent.ProbeFailed);

        Assert.Equal(DaemonState.Degraded, next.State);
        Assert.Equal(DaemonEffect.None, effect);
        Assert.Equal("health probe failed", next.LastFailureReason);
    }

    [Fact]
    public void Recovery_resubscribes_client_surfaces()
    {
        var degraded = DaemonSnapshot.Initial with { State = DaemonState.Degraded };

        var (next, effect) =
            DaemonSupervisorMachine.Reduce(degraded, DaemonEvent.ProbeSucceeded);

        Assert.Equal(DaemonState.Healthy, next.State);
        Assert.Equal(DaemonEffect.ResubscribeSurfaces, effect);
    }
}
