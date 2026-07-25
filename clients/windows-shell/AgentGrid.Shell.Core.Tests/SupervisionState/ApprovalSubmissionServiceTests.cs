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

    /// <summary>
    /// RED-then-GREEN (debug-run-shell-zero-sessions round 2, fix B): pins
    /// per-subscriber isolation on the SAME producer pattern as
    /// DaemonSupervisorService.Raise, on the OTHER confirmed/suspected
    /// producer (ApprovalSubmissionService.Apply). Proxies
    /// AppNotificationToastService's unmarshaled `root.SnapshotChanged +=
    /// snap => _ = _router.OnSnapshotAsync(snap)` wiring (AppNotificationToastService.cs:39
    /// pre-fix) — a throwing subscriber must not stop delivery to a
    /// well-behaved sibling subscriber, nor propagate out of Apply/
    /// SubmitApprovalAsync and abort the caller (the WS receive loop).
    /// </summary>
    [Fact]
    public async Task Apply_isolates_a_throwing_subscriber_from_others_and_from_its_caller()
    {
        var http = new FakeHttp(HttpStatusCode.OK);
        var svc = MakeService(http);

        var wellBehavedCalls = 0;
        svc.SnapshotChanged += _ =>
            throw new InvalidOperationException(
                "simulated off-UI-thread WinUI state read (AppNotificationToastService/SupervisionToastRouter)");
        svc.SnapshotChanged += _ => wellBehavedCalls++;

        // Apply must not throw even though the first subscriber always throws.
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        Assert.True(
            wellBehavedCalls > 0,
            "A non-throwing SnapshotChanged subscriber must still be notified " +
            "when a sibling subscriber throws.");

        // The same must hold for the async path (SubmitApprovalAsync calls
        // Apply internally at each transition) — a subscriber exception must
        // not unwind into the caller of SubmitApprovalAsync (e.g. the WS
        // receive loop), matching ShellCompositionRootTests' StartAsync guard
        // for the DaemonSupervisorService producer.
        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");

        Assert.True(wellBehavedCalls > 1);
        Assert.Empty(svc.Snapshot.Approvals);
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
