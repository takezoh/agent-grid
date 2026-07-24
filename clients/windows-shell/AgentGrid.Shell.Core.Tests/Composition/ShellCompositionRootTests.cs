using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.WorkspaceLauncher;

namespace AgentGrid.Shell.Core.Tests.Composition;

public class ShellCompositionRootTests
{
    [Fact]
    public async Task Build_and_start_with_fakes_reaches_healthy()
    {
        var http = new ScriptedHttp(() =>
            new HttpResponseMessage(System.Net.HttpStatusCode.OK)
            {
                Content = new StringContent("[]", System.Text.Encoding.UTF8, "application/json"),
            });
        using var httpClient = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:9") };

        var runner = new FakeRunner();
        var root = ShellCompositionRoot.Build(new ShellHostOptions
        {
            GatewayBaseUri = new Uri("http://127.0.0.1:9"),
            TokenPath = Path.Combine(Path.GetTempPath(), $"ag-t-{Guid.NewGuid():N}"),
            TokenSource = new StaticToken("tok"),
            HttpClient = httpClient,
            DaemonRunner = runner,
            Win32 = new NullWin32InteropService(),
            WorkspaceControlClient = new AlwaysOkPipe(),
            WorkspaceProcessLauncher = new NoopProcess(),
            WebSocketFactory = new ClosedWsFactory(),
            QuitApplication = () => { },
        });

        await using (root)
        {
            // Probe succeeds via scripted HTTP → Adopted without spawn.
            await root.Supervisor.StartAsync();
            Assert.True(root.Supervisor.Snapshot.IsConnected);
            Assert.Equal(TrayIconKind.Connected, TrayAppearanceMapper.From(root.Supervisor.Snapshot).Kind);
            Assert.NotNull(root.CurrentGlance());
        }
    }

    private sealed class StaticToken : ITokenSource
    {
        private readonly string _t;
        public StaticToken(string t) => _t = t;
        public Task<string> ReadFreshAsync(CancellationToken ct = default) => Task.FromResult(_t);
    }

    private sealed class ScriptedHttp : HttpMessageHandler
    {
        private readonly Func<HttpResponseMessage> _fn;
        public ScriptedHttp(Func<HttpResponseMessage> fn) => _fn = fn;
        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request, CancellationToken cancellationToken) =>
            Task.FromResult(_fn());
    }

    private sealed class FakeRunner : IDaemonRunner
    {
        public Task<SpawnResult> SpawnAsync(CancellationToken ct = default) =>
            Task.FromResult(new SpawnResult(SpawnResultKind.Started));
        public Task ShutdownAsync(CancellationToken ct = default) => Task.CompletedTask;
    }

    private sealed class AlwaysOkPipe : IWorkspaceControlClient
    {
        public Task<ControlReply> SendAsync(ControlEnvelope envelope, CancellationToken ct = default) =>
            Task.FromResult(ControlReply.Success());
    }

    private sealed class NoopProcess : IWorkspaceProcessLauncher
    {
        public Task SpawnAsync(CancellationToken ct = default) => Task.CompletedTask;
    }

    private sealed class ClosedWsFactory : IWebSocketTransportFactory
    {
        public IWebSocketTransport Create() => new ClosedWs();
    }

    private sealed class ClosedWs : IWebSocketTransport
    {
        public Task ConnectAsync(Uri uri, CancellationToken ct = default) => Task.CompletedTask;
        public Task<string?> ReceiveTextAsync(CancellationToken ct = default) =>
            Task.FromResult<string?>(null);
        public Task CloseAsync(CancellationToken ct = default) => Task.CompletedTask;
        public ValueTask DisposeAsync() => ValueTask.CompletedTask;
    }
}
