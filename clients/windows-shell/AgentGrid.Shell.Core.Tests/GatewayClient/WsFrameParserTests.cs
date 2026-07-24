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
    public void Ignores_output_array_and_unknown()
    {
        Assert.Null(WsFrameParser.TryParse("""[0,"o","Zg==","s1"]"""));
        Assert.Null(WsFrameParser.TryParse("""{"k":"h","protocolVersion":"1"}"""));
        Assert.Null(WsFrameParser.TryParse("not-json"));
        Assert.Null(WsFrameParser.TryParse(""));
    }
}
