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
        // rejects counterexample: card removed locally without ever issuing the POST
        // (the agent would stay blocked). AssertApprovalPost alone cannot catch this
        // because it only runs for requests that are actually sent.
        AssertRequestCount(http, 1);
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
        // rejects counterexample: a short-circuit that skips the POST entirely while
        // offline (leaving the card in place without ever contacting the gateway)
        // would also leave exactly one card here. Requests++ runs before the throw
        // in FakeHttp.SendAsync, so this distinguishes "tried and failed" from
        // "never tried".
        AssertRequestCount(http, 1);
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
        // witness-race-loss-normal: Handled elsewhere notice, not a blocking error dialog
        var glance = PanelGlanceView.From(svc.Snapshot);
        Assert.Empty(glance.Pending);
        Assert.Single(glance.Notices);
        Assert.Equal("already-handled", glance.Notices[0].Kind);
        Assert.Equal("Handled elsewhere", PanelPresentation.KindLabel(glance.Notices[0].Kind));
    }

    // witness-submit-normal: optimistic removal is visible before the HTTP response
    // (rejects counterexample: spinner/blocked UI waiting on 2xx).
    [Fact]
    public async Task Submit_removes_card_before_http_response_completes()
    {
        var gate = new TaskCompletionSource<HttpResponseMessage>(
            TaskCreationOptions.RunContinuationsAsynchronously);
        var http = new GatedHttp(gate.Task);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));
        // Precondition: the card must exist, else the poll below exits immediately
        // and Assert.Empty would pass without optimistic removal ever happening.
        Assert.Single(svc.Snapshot.Approvals);

        var submit = svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");
        // Yield until optimistic Apply has run (HTTP still blocked).
        for (var i = 0; i < 50 && svc.Snapshot.Approvals.Count > 0; i++)
            await Task.Delay(10);
        Assert.Empty(svc.Snapshot.Approvals);
        Assert.Empty(svc.Snapshot.AlreadyHandled);

        gate.SetResult(new HttpResponseMessage(HttpStatusCode.OK)
        {
            Content = new StringContent("{}", Encoding.UTF8, "application/json"),
        });
        await submit;
        Assert.Empty(svc.Snapshot.Approvals);
        // The optimistic removal must not replace the request: exactly one POST issued.
        AssertRequestCount(http, 1);
    }

    // witness-lifecycle-adversarial: restore → resubmit → delayed 409 normalizes
    // to Handled elsewhere; card count stays ≤ 1; silent failure forbidden.
    [Fact]
    public async Task Rollback_then_resubmit_conflict_normalizes_to_handled_elsewhere()
    {
        var http = new SequenceHttp(
            new HttpRequestException("network down"),
            HttpStatusCode.Conflict);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");
        Assert.Single(svc.Snapshot.Approvals); // restored
        Assert.Empty(svc.Snapshot.AlreadyHandled);

        // Re-issue of same id must not stack a second card (flood + lifecycle).
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));
        Assert.Single(svc.Snapshot.Approvals);

        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");
        Assert.Empty(svc.Snapshot.Approvals);
        Assert.Single(svc.Snapshot.AlreadyHandled);
        Assert.Equal("a1", svc.Snapshot.AlreadyHandled[0].ItemId);
    }

    // witness-offline-rollback-normal: restored card coexists with offline badge.
    [Fact]
    public async Task Network_error_restores_card_and_connection_failure_surfaces_offline()
    {
        var http = new FakeHttp(throwNetwork: true);
        var svc = MakeService(http);
        svc.Apply(new EvtApprovalRequested("a1", "s1", "cmd"));

        await svc.SubmitApprovalAsync("a1", "s1", "accept", "cmd");
        Assert.Single(svc.Snapshot.Approvals); // not silent failure

        svc.Apply(new EvtConnectionFailed("ws down"));
        var glance = PanelGlanceView.From(svc.Snapshot);
        Assert.True(glance.ConnectionFailed);
        Assert.Single(glance.Pending);
        var sections = PanelPresentation.Sections(glance);
        Assert.True(sections.ShowOffline);
        Assert.True(sections.ShowPending);
        // Compact prioritizes offline badge over pending count.
        var compact = PanelPresentation.Compact(glance);
        Assert.Equal("offline", compact.Text);
        Assert.Equal("danger", compact.AccentToken);
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

    private static ApprovalSubmissionService MakeService(HttpMessageHandler handler)
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

    // UAC-002 counterexample rejection: 「ローカルで消すだけの実装はセッションが待機の
    // ままになることで fail する」.
    //
    // Two halves are needed and neither alone is sufficient:
    //   (a) AssertApprovalPost validates the shape of a request that IS issued. It runs
    //       inside SendAsync, so by construction it can never fire for an implementation
    //       that issues no request at all.
    //   (b) IRequestCounter.Requests records whether a request was issued. Tests that
    //       claim the "removed the card but never POSTed" counterexample is rejected must
    //       assert this counter — that is the half with the discriminating power.
    // Mirrors QuestionSubmissionServiceTests' Scripted handler, which asserts the
    // /questions/ path.
    private interface IRequestCounter
    {
        int Requests { get; }
    }

    // Referencing the interface as a type (not just the concrete Requests property)
    // means a future fake that forgets to implement IRequestCounter fails to compile
    // at the call site, instead of silently never being asked the question.
    private static void AssertRequestCount(IRequestCounter counter, int expected)
    {
        Assert.Equal(expected, counter.Requests);
    }

    private static void AssertApprovalPost(HttpRequestMessage request)
    {
        if (request.Method != HttpMethod.Post)
            throw new InvalidOperationException($"expected POST, got {request.Method}");
        var path = request.RequestUri!.AbsolutePath;
        if (!path.Contains("/approvals/", StringComparison.Ordinal))
            throw new InvalidOperationException($"expected an /approvals/ path, got {path}");
        var body = request.Content?.ReadAsStringAsync().GetAwaiter().GetResult() ?? "";
        if (!body.Contains("\"decision\"", StringComparison.Ordinal))
            throw new InvalidOperationException($"expected a decision in the body, got {body}");
    }

    private sealed class FakeHttp : HttpMessageHandler, IRequestCounter
    {
        public int Requests { get; private set; }

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
            AssertApprovalPost(request);
            Requests++;
            if (_throwNetwork)
                throw new HttpRequestException("network down");
            return Task.FromResult(new HttpResponseMessage(_code)
            {
                Content = new StringContent("{}", Encoding.UTF8, "application/json"),
            });
        }
    }

    private sealed class GatedHttp : HttpMessageHandler, IRequestCounter
    {
        public int Requests { get; private set; }

        private readonly Task<HttpResponseMessage> _response;
        public GatedHttp(Task<HttpResponseMessage> response) => _response = response;

        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request,
            CancellationToken cancellationToken)
        {
            AssertApprovalPost(request);
            Requests++;
            return _response;
        }
    }

    private sealed class SequenceHttp : HttpMessageHandler, IRequestCounter
    {
        public int Requests { get; private set; }

        private readonly object[] _steps;
        private int _i;

        public SequenceHttp(params object[] steps) => _steps = steps;

        protected override Task<HttpResponseMessage> SendAsync(
            HttpRequestMessage request,
            CancellationToken cancellationToken)
        {
            AssertApprovalPost(request);
            Requests++;
            var step = _steps[_i++];
            if (step is Exception ex)
                throw ex;
            var code = (HttpStatusCode)step;
            return Task.FromResult(new HttpResponseMessage(code)
            {
                Content = new StringContent("{}", Encoding.UTF8, "application/json"),
            });
        }
    }
}
