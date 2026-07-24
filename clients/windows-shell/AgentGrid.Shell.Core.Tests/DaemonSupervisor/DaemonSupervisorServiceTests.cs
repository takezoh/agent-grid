using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

public class DaemonSupervisorServiceTests
{
    [Fact]
    public async Task Start_adopts_when_probe_ok()
    {
        var runner = new FakeRunner();
        var probe = new FakeProbe(succeed: true);
        var svc = new DaemonSupervisorService(runner, probe);

        await svc.StartAsync();

        Assert.Equal(DaemonState.Adopted, svc.Snapshot.State);
        Assert.Equal(0, runner.SpawnCalls);
    }

    [Fact]
    public async Task Start_spawns_when_probe_fails_then_healthy()
    {
        var runner = new FakeRunner { SpawnResult = new SpawnResult(SpawnResultKind.Started) };
        var probe = new FakeProbe();
        // First probe (adopt) fails; after spawn, probe succeeds.
        probe.Results.Enqueue(false);
        probe.Results.Enqueue(true);
        var svc = new DaemonSupervisorService(runner, probe);

        await svc.StartAsync();

        Assert.Equal(DaemonState.Healthy, svc.Snapshot.State);
        Assert.Equal(1, runner.SpawnCalls);
    }

    [Fact]
    public async Task Spawn_crash_degrades_without_second_spawn()
    {
        var runner = new FakeRunner
        {
            SpawnResult = new SpawnResult(SpawnResultKind.ExitedImmediately, "boom"),
        };
        var probe = new FakeProbe(succeed: false);
        var svc = new DaemonSupervisorService(runner, probe);

        await svc.StartAsync();

        Assert.Equal(DaemonState.Degraded, svc.Snapshot.State);
        Assert.Equal(1, runner.SpawnCalls);
    }

    [Fact]
    public async Task StopDaemon_invokes_shutdown()
    {
        var runner = new FakeRunner { SpawnResult = new SpawnResult(SpawnResultKind.Started) };
        var probe = new FakeProbe();
        probe.Results.Enqueue(false);
        probe.Results.Enqueue(true);
        var svc = new DaemonSupervisorService(runner, probe);
        await svc.StartAsync();
        Assert.True(svc.Snapshot.IsConnected);

        await svc.StopDaemonAsync();

        Assert.Equal(DaemonState.NotRunning, svc.Snapshot.State);
        Assert.Equal(1, runner.ShutdownCalls);
    }

    private sealed class FakeRunner : IDaemonRunner
    {
        public int SpawnCalls { get; private set; }
        public int ShutdownCalls { get; private set; }
        public SpawnResult SpawnResult { get; set; } = new(SpawnResultKind.Started);

        public Task<SpawnResult> SpawnAsync(CancellationToken ct = default)
        {
            SpawnCalls++;
            return Task.FromResult(SpawnResult);
        }

        public Task ShutdownAsync(CancellationToken ct = default)
        {
            ShutdownCalls++;
            return Task.CompletedTask;
        }
    }

    private sealed class FakeProbe : IDaemonHealthProbe
    {
        private readonly bool _default;
        public Queue<bool> Results { get; } = new();

        public FakeProbe(bool succeed = true) => _default = succeed;

        public Task<bool> ProbeAsync(CancellationToken ct = default)
        {
            if (Results.Count > 0)
                return Task.FromResult(Results.Dequeue());
            return Task.FromResult(_default);
        }
    }
}
