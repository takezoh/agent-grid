using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

public class DaemonSupervisorMachineTests
{
    [Fact]
    public void Start_triggers_probe_not_spawn()
    {
        var (next, effect) = DaemonSupervisorMachine.Reduce(DaemonSnapshot.Initial, DaemonEvent.Start);
        Assert.Equal(DaemonState.NotRunning, next.State);
        Assert.Equal(DaemonEffect.Probe, effect);
    }

    [Fact]
    public void Probe_success_on_boot_adopts()
    {
        var snap = DaemonSnapshot.Initial;
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.Start);
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeSucceeded);
        Assert.Equal(DaemonState.Adopted, snap.State);
        Assert.True(snap.IsConnected);
        Assert.Equal(DaemonEffect.None, effect);
        Assert.False(snap.HasSpawnedThisBoot);
    }

    [Fact]
    public void Probe_failure_spawns_once()
    {
        var snap = DaemonSnapshot.Initial;
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.Start);
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeFailed);
        Assert.Equal(DaemonState.Spawning, snap.State);
        Assert.Equal(DaemonEffect.Spawn, effect);
    }

    [Fact]
    public void Spawned_then_healthy()
    {
        var snap = GoToSpawning();
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.SpawnStarted);
        Assert.True(snap.HasSpawnedThisBoot);
        Assert.Equal(1, snap.SpawnCountThisBoot);
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeSucceeded);
        Assert.Equal(DaemonState.Healthy, snap.State);
        Assert.True(snap.IsConnected);
    }

    [Fact]
    public void Spawn_exit_immediate_degrades_without_second_spawn()
    {
        var snap = GoToSpawning();
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.SpawnExitedImmediately);
        Assert.Equal(DaemonState.Degraded, snap.State);
        Assert.Equal(DaemonEffect.None, effect);
        Assert.False(snap.IsConnected);

        // A subsequent Start-style probe failure must NOT spawn again this boot.
        // From Degraded, HealthTick probes; ProbeFailed stays Degraded (no spawn).
        (snap, effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.HealthTick);
        Assert.Equal(DaemonEffect.Probe, effect);
        (snap, effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeFailed);
        Assert.Equal(DaemonState.Degraded, snap.State);
        Assert.Equal(DaemonEffect.None, effect);
    }

    [Fact]
    public void No_second_concurrent_spawn_after_spawn_failed()
    {
        var snap = GoToSpawning();
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.SpawnFailed);
        Assert.Equal(DaemonState.Degraded, snap.State);

        // Even Start from NotRunning path isn't re-entered; Degraded refuses spawn.
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.Start);
        Assert.Equal(DaemonEffect.None, effect);
    }

    [Fact]
    public void Healthy_probe_failure_degrades()
    {
        var snap = GoToHealthy();
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeFailed);
        Assert.Equal(DaemonState.Degraded, snap.State);
        Assert.False(snap.IsConnected);
    }

    [Fact]
    public void Restart_swaps_then_respawns()
    {
        var snap = GoToHealthy();
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.RestartRequested);
        Assert.Equal(DaemonState.Swapping, snap.State);
        Assert.Equal(DaemonEffect.Shutdown, effect);

        (snap, effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ShutdownCompleted);
        Assert.Equal(DaemonState.Spawning, snap.State);
        Assert.Equal(DaemonEffect.Spawn, effect);
        Assert.False(snap.HasSpawnedThisBoot);
    }

    [Fact]
    public void Stop_returns_to_not_running()
    {
        var snap = GoToHealthy();
        (snap, var effect) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.StopRequested);
        Assert.Equal(DaemonState.NotRunning, snap.State);
        Assert.Equal(DaemonEffect.Shutdown, effect);
        Assert.False(snap.IsConnected);
    }

    [Theory]
    [InlineData(DaemonState.NotRunning, false)]
    [InlineData(DaemonState.Spawning, false)]
    [InlineData(DaemonState.Degraded, false)]
    [InlineData(DaemonState.Swapping, false)]
    [InlineData(DaemonState.Healthy, true)]
    [InlineData(DaemonState.Adopted, true)]
    public void IsConnected_partition(DaemonState state, bool expected)
    {
        var snap = DaemonSnapshot.Initial with { State = state };
        Assert.Equal(expected, snap.IsConnected);
    }

    private static DaemonSnapshot GoToSpawning()
    {
        var snap = DaemonSnapshot.Initial;
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.Start);
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeFailed);
        return snap;
    }

    private static DaemonSnapshot GoToHealthy()
    {
        var snap = GoToSpawning();
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.SpawnStarted);
        (snap, _) = DaemonSupervisorMachine.Reduce(snap, DaemonEvent.ProbeSucceeded);
        return snap;
    }
}
