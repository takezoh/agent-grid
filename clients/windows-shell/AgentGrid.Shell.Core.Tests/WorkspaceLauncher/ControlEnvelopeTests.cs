using AgentGrid.Shell.Core.WorkspaceLauncher;

namespace AgentGrid.Shell.Core.Tests.WorkspaceLauncher;

public class ControlEnvelopeTests
{
    [Theory]
    [InlineData("{\"op\":\"openSession\",\"server_id\":\"local\",\"session_id\":\"sess-1\"}")]
    [InlineData("{\"op\":\"activate\"}")]
    [InlineData("{\"op\":\"quit\"}")]
    [InlineData("{\"op\":\"openSession\",\"server_id\":\"local\",\"session_id\":\"s\",\"schema_version\":2}")]
    public void Accepts_closed_schema(string line)
    {
        var r = ControlEnvelope.ParseLine(line);
        Assert.True(r.Success, r.Error);
        Assert.NotNull(r.Envelope);
    }

    [Theory]
    [InlineData("{\"op\":\"openSession\",\"server_id\":\"local\",\"session_id\":\"s\",\"extra\":1}", "unknown field")]
    [InlineData("{\"op\":\"openSession\",\"server_id\":\"local\",\"session_id\":\"s\",\"health\":\"ok\"}", "unknown field")]
    [InlineData("{\"op\":\"nope\"}", "unknown op")]
    [InlineData("{\"op\":\"openSession\"}", "requires server_id")]
    [InlineData("not-json", "malformed")]
    [InlineData("", "empty")]
    public void Rejects_unknown_fields_and_ops(string line, string expectedFragment)
    {
        var r = ControlEnvelope.ParseLine(line);
        Assert.False(r.Success);
        Assert.Contains(expectedFragment, r.Error!, StringComparison.OrdinalIgnoreCase);
    }

    [Fact]
    public void Round_trip_json_line()
    {
        var env = new ControlEnvelope
        {
            Op = "openSession",
            ServerId = "local",
            SessionId = "sess-x",
        };
        var line = env.ToJsonLine();
        var r = ControlEnvelope.ParseLine(line);
        Assert.True(r.Success);
        Assert.Equal("openSession", r.Envelope!.Op);
        Assert.Equal("local", r.Envelope.ServerId);
        Assert.Equal("sess-x", r.Envelope.SessionId);
    }

    [Fact]
    public void Reply_shapes()
    {
        Assert.Contains("\"ok\":true", ControlReply.Success().ToJsonLine(), StringComparison.Ordinal);
        var fail = ControlReply.Fail("boom").ToJsonLine();
        Assert.Contains("\"ok\":false", fail, StringComparison.Ordinal);
        Assert.Contains("boom", fail, StringComparison.Ordinal);
    }
}
