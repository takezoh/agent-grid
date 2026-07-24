using AgentGrid.Shell.Core.DaemonSupervisor;

namespace AgentGrid.Shell.Core.Tests.DaemonSupervisor;

public class WslDaemonRunnerTests
{
    [Fact]
    public void Detach_command_uses_setsid_nohup()
    {
        var runner = new WslDaemonRunner("Ubuntu", "~/agent-grid/server", 8443);
        var cmd = runner.BuildDetachCommand();
        Assert.Contains(WslDetachNotes.CandidateDetachPrefix, cmd, StringComparison.Ordinal);
        Assert.Contains("127.0.0.1:8443", cmd, StringComparison.Ordinal);
        Assert.Contains("token-file", cmd, StringComparison.Ordinal);
        Assert.Contains("-data-dir", cmd, StringComparison.Ordinal);
        Assert.Contains("-insecure", cmd, StringComparison.Ordinal);
        Assert.Contains("echo $!", cmd, StringComparison.Ordinal);
    }

    [Fact]
    public async Task Spawn_uses_injected_runner()
    {
        string? seenFile = null;
        string? seenArgs = null;
        var runner = new WslDaemonRunner(
            "Ubuntu",
            "/opt/server",
            9000,
            run: (file, args) =>
            {
                seenFile = file;
                seenArgs = args;
                return Task.FromResult(0);
            });

        var result = await runner.SpawnAsync();
        Assert.Equal(SpawnResultKind.Started, result.Kind);
        Assert.Equal("wsl.exe", seenFile);
        Assert.Contains("setsid nohup", seenArgs, StringComparison.Ordinal);
        Assert.Contains("9000", seenArgs, StringComparison.Ordinal);
    }
}
