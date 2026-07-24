using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Menu;

namespace AgentGrid.Shell.Core.Tests.Menu;

/// <summary>
/// contract-quit-vs-daemon-stop: Quit never invokes Stop.
/// Lives under Core.Tests but exercises AgentGrid.Shell menu handlers
/// (project reference added for this invariant).
/// </summary>
public class ShellMenuHandlersTests
{
    [Fact]
    public async Task Quit_does_not_stop_daemon()
    {
        var runner = new CountingRunner();
        var probe = new AlwaysOkProbe();
        var svc = new DaemonSupervisorService(runner, probe);
        // Adopt to Healthy-ish Connected state first.
        await svc.StartAsync();
        Assert.True(svc.Snapshot.IsConnected);

        var quitCalled = false;
        var handlers = new ShellMenuHandlers(svc, () => quitCalled = true);
        handlers.OnQuit();

        Assert.True(quitCalled);
        Assert.Equal(0, runner.ShutdownCalls);
        // Daemon remains connected after Quit.
        Assert.True(svc.Snapshot.IsConnected);
    }

    [Fact]
    public async Task StopDaemon_menu_invokes_shutdown()
    {
        var runner = new CountingRunner();
        var probe = new AlwaysOkProbe();
        var svc = new DaemonSupervisorService(runner, probe);
        await svc.StartAsync();

        var handlers = new ShellMenuHandlers(svc, () => { });
        await handlers.OnStopDaemonAsync();

        Assert.Equal(1, runner.ShutdownCalls);
        Assert.Equal(DaemonState.NotRunning, svc.Snapshot.State);
    }

    private sealed class CountingRunner : IDaemonRunner
    {
        public int ShutdownCalls { get; private set; }
        public Task<SpawnResult> SpawnAsync(CancellationToken ct = default) =>
            Task.FromResult(new SpawnResult(SpawnResultKind.Started));
        public Task ShutdownAsync(CancellationToken ct = default)
        {
            ShutdownCalls++;
            return Task.CompletedTask;
        }
    }

    private sealed class AlwaysOkProbe : IDaemonHealthProbe
    {
        public Task<bool> ProbeAsync(CancellationToken ct = default) => Task.FromResult(true);
    }
}
