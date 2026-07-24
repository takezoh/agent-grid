using System.Collections.Concurrent;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.GatewayClient;

public class SupervisionWsSessionTests
{
    [Fact]
    public async Task RunOnce_applies_frames_and_restored()
    {
        var http = new ScriptedHttpHandler(req =>
        {
            if (req.RequestUri!.AbsolutePath.EndsWith("/api/ws-ticket", StringComparison.Ordinal))
            {
                return JsonResponse(200, """{"ticket":"t-1","client_instance_id":"ci-1"}""");
            }
            throw new InvalidOperationException(req.RequestUri.ToString());
        });
        using var clientHttp = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:8443") };
        var tokens = new StaticToken("tok");
        var gateway = new ShellGatewayClient(new Uri("http://127.0.0.1:8443"), tokens, clientHttp);
        var supervision = new ApprovalSubmissionService(gateway);

        var frames = new ConcurrentQueue<string>();
        frames.Enqueue("""{"k":"ar","approval":{"id":"ap-1","session_id":"s1","status":"pending","command":"ls"}}""");
        frames.Enqueue("""{"k":"v","sessions":[{"id":"s1","project":"p","command":"c","state":"waiting","view":{"card":{"title":"X"}}}]}""");

        var factory = new ScriptedWsFactory(frames);
        var session = new SupervisionWsSession(gateway, supervision, factory, backoff: _ => TimeSpan.Zero);

        await session.RunOnceAsync();

        Assert.False(supervision.Snapshot.ConnectionFailed);
        Assert.Single(supervision.Snapshot.Approvals);
        Assert.Equal("ap-1", supervision.Snapshot.Approvals[0].ApprovalId);
        Assert.Single(supervision.Snapshot.Sessions);
        Assert.Equal(SessionPhase.Waiting, supervision.Snapshot.Sessions[0].Phase);
    }

    [Fact]
    public async Task RunOnce_records_connection_failed_on_mint_error()
    {
        var http = new ScriptedHttpHandler(_ => JsonResponse(500, "{}"));
        using var clientHttp = new HttpClient(http) { BaseAddress = new Uri("http://127.0.0.1:8443") };
        var gateway = new ShellGatewayClient(new Uri("http://127.0.0.1:8443"), new StaticToken("t"), clientHttp);
        var supervision = new ApprovalSubmissionService(gateway);
        var session = new SupervisionWsSession(
            gateway,
            supervision,
            new ScriptedWsFactory(new ConcurrentQueue<string>()),
            backoff: _ => TimeSpan.Zero);

        await Assert.ThrowsAnyAsync<Exception>(() => session.RunOnceAsync());
        Assert.True(supervision.Snapshot.ConnectionFailed);
    }

    private sealed class StaticToken : ITokenSource
    {
        private readonly string _t;
        public StaticToken(string t) => _t = t;
        public Task<string> ReadFreshAsync(CancellationToken ct = default) => Task.FromResult(_t);
    }

    private sealed class ScriptedHttpHandler : HttpMessageHandler
    {
        private readonly Func<HttpRequestMessage, HttpResponseMessage> _fn;
        public ScriptedHttpHandler(Func<HttpRequestMessage, HttpResponseMessage> fn) => _fn = fn;
        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request,
            CancellationToken cancellationToken) =>
            Task.FromResult(_fn(request));
    }

    private static HttpResponseMessage JsonResponse(int status, string body)
    {
        var res = new HttpResponseMessage((System.Net.HttpStatusCode)status)
        {
            Content = new StringContent(body, System.Text.Encoding.UTF8, "application/json"),
        };
        return res;
    }

    private sealed class ScriptedWsFactory : IWebSocketTransportFactory
    {
        private readonly ConcurrentQueue<string> _frames;
        public ScriptedWsFactory(ConcurrentQueue<string> frames) => _frames = frames;
        public IWebSocketTransport Create() => new ScriptedWs(_frames);
    }

    private sealed class ScriptedWs : IWebSocketTransport
    {
        private readonly ConcurrentQueue<string> _frames;
        public ScriptedWs(ConcurrentQueue<string> frames) => _frames = frames;
        public Task ConnectAsync(Uri uri, CancellationToken ct = default) => Task.CompletedTask;
        public Task<string?> ReceiveTextAsync(CancellationToken ct = default) =>
            Task.FromResult(_frames.TryDequeue(out var f) ? f : null);
        public Task CloseAsync(CancellationToken ct = default) => Task.CompletedTask;
        public ValueTask DisposeAsync() => ValueTask.CompletedTask;
    }
}
