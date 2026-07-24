using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

/// <summary>
/// unit-restart-continuity-integration (S5): restart drives shutdown → spawn → resubscribe.
/// </summary>
public class RestartContinuityTests
{
    [Fact]
    public async Task Restart_resubscribes_surfaces()
    {
        var runner = new SeqRunner();
        var probe = new SeqProbe();
        // adopt fail → spawn → healthy
        probe.Results.Enqueue(false);
        probe.Results.Enqueue(true);
        // post-restart probe success
        probe.Results.Enqueue(true);

        var resub = new CountingResub();
        var svc = new DaemonSupervisorService(runner, probe, resub);
        await svc.StartAsync();
        Assert.Equal(DaemonState.Healthy, svc.Snapshot.State);

        await svc.RestartAsync();

        Assert.True(svc.Snapshot.IsConnected);
        Assert.True(runner.ShutdownCalls >= 1);
        Assert.True(runner.SpawnCalls >= 2); // initial + restart
        // Resubscribe on every Spawning→Healthy (cold spawn + post-restart).
        Assert.Equal(2, resub.Calls);
    }

    private sealed class SeqRunner : IDaemonRunner
    {
        public int SpawnCalls { get; private set; }
        public int ShutdownCalls { get; private set; }
        public Task<SpawnResult> SpawnAsync(CancellationToken ct = default)
        {
            SpawnCalls++;
            return Task.FromResult(new SpawnResult(SpawnResultKind.Started));
        }
        public Task ShutdownAsync(CancellationToken ct = default)
        {
            ShutdownCalls++;
            return Task.CompletedTask;
        }
    }

    private sealed class SeqProbe : IDaemonHealthProbe
    {
        public Queue<bool> Results { get; } = new();
        public Task<bool> ProbeAsync(CancellationToken ct = default) =>
            Task.FromResult(Results.Count > 0 ? Results.Dequeue() : true);
    }

    private sealed class CountingResub : ISurfaceResubscriber
    {
        public int Calls { get; private set; }
        public Task ResubscribeAllAsync(CancellationToken ct = default)
        {
            Calls++;
            return Task.CompletedTask;
        }
    }
}
