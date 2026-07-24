using System.Net;
using System.Text;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.SupervisionState;

public class ApprovalSubmissionServiceTests
{
    [Fact]
    public async Task Success_leaves_queue_empty()
    {
        var http = new FakeHttp(HttpStatusCode.OK);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");

        Assert.Empty(svc.Snapshot.Approvals);
        Assert.Empty(svc.Snapshot.AlreadyHandled);
    }

    [Fact]
    public async Task Network_error_rolls_back()
    {
        var http = new FakeHttp(throwNetwork: true);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");

        Assert.Single(svc.Snapshot.Approvals);
        Assert.Equal("a1", svc.Snapshot.Approvals[0].ApprovalId);
    }

    [Fact]
    public async Task Conflict_shows_resolved_by_other()
    {
        var http = new FakeHttp(HttpStatusCode.Conflict);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        await svc.SubmitApprovalAsync("a1", "s1", "deny", "cmd");

        Assert.Empty(svc.Snapshot.Approvals);
        Assert.Single(svc.Snapshot.AlreadyHandled);
    }

    private static ApprovalSubmissionService MakeService(FakeHttp handler)
    {
        var client = new HttpClient(handler) { BaseAddress = new Uri("http://localhost/") };
        var tokens = new StaticToken("t");
        var gw = new ShellGatewayClient(new Uri("http://localhost/"), tokens, client);
        return new ApprovalSubmissionService(gw);
    }

    private sealed class StaticToken : ITokenSource
    {
        private readonly string _t;
        public StaticToken(string t) => _t = t;
        public Task<string> ReadFreshAsync(CancellationToken ct = default) => Task.FromResult(_t);
    }

    private sealed class FakeHttp : HttpMessageHandler
    {
        private readonly HttpStatusCode _code;
        private readonly bool _throwNetwork;

        public FakeHttp(HttpStatusCode code = HttpStatusCode.OK, bool throwNetwork = false)
        {
            _code = code;
            _throwNetwork = throwNetwork;
        }

        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request,
            CancellationToken cancellationToken)
        {
            if (_throwNetwork)
                throw new HttpRequestException("network down");
            return Task.FromResult(new HttpResponseMessage(_code)
            {
                Content = new StringContent("{}", Encoding.UTF8, "application/json"),
            });
        }
    }
}
