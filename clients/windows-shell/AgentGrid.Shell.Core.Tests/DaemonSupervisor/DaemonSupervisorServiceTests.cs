using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

public sealed class DaemonSupervisorServiceTests
{
    [Fact]
    public async Task Start_probes_and_reports_connected()
    {
        var service = new DaemonSupervisorService(new FixedProbe(true));

        await service.StartAsync();

        Assert.Equal(DaemonState.Healthy, service.Snapshot.State);
        Assert.True(service.Snapshot.IsConnected);
    }

    [Fact]
    public async Task Failed_probe_reports_degraded_without_starting_a_server()
    {
        var service = new DaemonSupervisorService(new FixedProbe(false));

        await service.StartAsync();

        Assert.Equal(DaemonState.Degraded, service.Snapshot.State);
        Assert.False(service.Snapshot.IsConnected);
    }

    private sealed class FixedProbe(bool result) : IDaemonHealthProbe
    {
        public Task<bool> ProbeAsync(CancellationToken ct = default) =>
            Task.FromResult(result);
    }
}
