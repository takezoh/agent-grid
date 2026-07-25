using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.Tests.GatewayClient;

public class WsFrameParserTests
{
    [Fact]
    public void Parses_approval_requested_fixture()
    {
        var json = """
            {"k":"ar","approval":{"id":"ap-fixed-1","session_id":"sess-fixed-1","frame_id":"frame-fixed-1","kind":"command","command":"echo hello","reason":"fixture","created_at":"2026-07-23T00:00:00.000Z","expires_at":"2026-07-23T00:00:30.000Z","status":"pending","default_decision":"deny"}}
            """;
        var ev = WsFrameParser.TryParse(json) as EvtApprovalRequested;
        Assert.NotNull(ev);
        Assert.Equal("ap-fixed-1", ev!.ApprovalId);
        Assert.Equal("sess-fixed-1", ev.SessionId);
        Assert.Equal("echo hello", ev.Summary);
        Assert.NotNull(ev.ExpiresAt);
    }

    [Fact]
    public void Parses_approval_resolved()
    {
        var json = """
            {"k":"ax","approval":{"id":"ap-1","session_id":"s1","status":"resolved","decision":"accept","resolving_client_instance_id":"ci-A"}}
            """;
        var ev = WsFrameParser.TryParse(json) as EvtApprovalResolved;
        Assert.NotNull(ev);
        Assert.Equal("accept", ev!.Decision);
        Assert.Equal("ci-A", ev.DecidedBy);
    }

    [Fact]
    public void Parses_question_requested()
    {
        var json = """
            {"k":"qr","question":{"id":"q1","session_id":"s1","status":"pending","prompt":"Continue?"}}
            """;
        var ev = WsFrameParser.TryParse(json) as EvtQuestionRequested;
        Assert.NotNull(ev);
        Assert.Equal("Continue?", ev!.Prompt);
    }

    [Fact]
    public void Parses_view_update_sessions()
    {
        var json = """
            {"k":"v","sessions":[{"id":"s1","project":"/repo/p","command":"claude","state":"running","view":{"card":{"title":"T1"}}}]}
            """;
        var ev = WsFrameParser.TryParse(json) as EvtViewUpdateSessions;
        Assert.NotNull(ev);
        Assert.Single(ev!.Sessions);
        Assert.Equal("s1", ev.Sessions[0].SessionId);
        Assert.Equal(SessionPhase.Running, ev.Sessions[0].Phase);
        Assert.Equal("T1", ev.Sessions[0].Title);
    }

    [Fact]
    public void View_update_does_not_promote_project_or_command_into_title()
    {
        var json = """
            {"k":"v","sessions":[{"id":"s1","project":"/repo/p","command":"claude","state":"running","view":{"card":{}}}]}
            """;

        var ev = Assert.IsType<EvtViewUpdateSessions>(WsFrameParser.TryParse(json));

        Assert.Null(Assert.Single(ev.Sessions).Title);
    }

    [Fact]
    public void Ignores_output_array_and_unknown()
    {
        Assert.Null(WsFrameParser.TryParse("""[0,"o","Zg==","s1"]"""));
        Assert.Null(WsFrameParser.TryParse("not-json"));
        Assert.Null(WsFrameParser.TryParse(""));
    }

    // RED (debug-run-shell-zero-sessions / hypothesis-hello-frame-dropped):
    // the gateway's k="h" hello frame is the ONLY frame that carries the
    // full session list at connect time when sessions already existed before
    // this client subscribed (server/api/gateway.go SubscribeLifecycle:
    // "the very first sessions-changed event arrives as a direct daemon-side
    // reaction to CmdSubscribe" and is encoded via encodeHelloFrame, not
    // encodeServerEvent — see lifecycleFrames' `if helloSent` branch and
    // TestGatewayLifecycle_EmitsHelloFirst in gateway_view_update_test.go).
    // WsFrameParser.TryParse's switch has no "h" arm, so it falls to the
    // default `_ => null` and the hello-seeded sessions are silently
    // dropped — SupervisionReducer never sees EvtViewUpdateSessions unless a
    // *subsequent* session-list change fires a "v" frame after this client
    // is already connected. This reproduces "notch panel always shows
    // 0 session(s)" for a shell that connects to a gateway with pre-existing
    // sessions and no session churn afterward.
    //
    // EXPECTED (currently FAILS): parsing a hello frame containing a
    // sessions array must yield the same EvtViewUpdateSessions the "v" frame
    // would, so the reducer can seed SupervisionSnapshot.Sessions from it.
    [Fact]
    public void Parses_hello_frame_seeds_initial_sessions()
    {
        var json = """
            {"k":"h","sessions":[{"id":"s1","project":"/repo/p","command":"claude","state":"running","view":{"card":{"title":"T1"}}}],"features":["surface"],"serverTime":1700000001,"protocolVersion":"1.0.0-phase01","capabilities":["sessions.view_update"]}
            """;
        var ev = WsFrameParser.TryParse(json) as EvtViewUpdateSessions;
        Assert.NotNull(ev);
        Assert.Single(ev!.Sessions);
        Assert.Equal("s1", ev.Sessions[0].SessionId);
        Assert.Equal(SessionPhase.Running, ev.Sessions[0].Phase);
        Assert.Equal("T1", ev.Sessions[0].Title);
    }

    // A hello frame that legitimately carries no session data (e.g. a
    // capability-negotiation-only variant) must still be ignorable — this
    // pins that the fix (routing "h" through the same sessions-array parse
    // as "v") does not turn every hello frame into a forced snapshot reset.
    [Fact]
    public void Ignores_hello_frame_without_sessions_key()
    {
        Assert.Null(WsFrameParser.TryParse("""{"k":"h","protocolVersion":"1"}"""));
    }
}
