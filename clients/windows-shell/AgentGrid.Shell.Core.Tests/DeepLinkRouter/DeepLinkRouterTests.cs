using AgentGrid.Shell.Core.DeepLinkRouter;
using Router = AgentGrid.Shell.Core.DeepLinkRouter.DeepLinkRouter;

namespace AgentGrid.Shell.Core.Tests.DeepLinkRouter;

public class DeepLinkRouterTests
{
    [Theory]
    [InlineData("agent-grid://server/one/session/sess-1", RouteKind.OpenWorkspaceSession, "sess-1", "session", "typed-helper")]
    [InlineData("agent-grid://server/one/approval/ap-1", RouteKind.PanelFocusItem, "ap-1", "approval", "typed-helper")]
    [InlineData("agent-grid://server/one/question/q-1", RouteKind.PanelFocusItem, "q-1", "question", "alias")]
    [InlineData("agent-grid://server/one/session/sess-1/jump", RouteKind.JumpBack, "sess-1", "session", "alias")]
    public void Routes_typed_and_alias(string uri, RouteKind kind, string id, string itemKind, string source)
    {
        var d = Router.Route(uri);
        Assert.Equal(kind, d.Kind);
        Assert.Equal("one", d.ServerId);
        Assert.Equal(id, d.Id);
        Assert.Equal(itemKind, d.ItemKind);
        Assert.Equal(source, d.Source);
    }

    [Theory]
    [InlineData("")]
    [InlineData("http://example.com")]
    [InlineData("agent-grid://unknown/x")]
    [InlineData("agent-grid://session/")]
    public void Rejects_malformed_or_unknown(string uri)
    {
        var d = Router.Route(uri);
        Assert.Equal(RouteKind.Rejected, d.Kind);
        Assert.Equal("rejected", d.Source);
        Assert.NotNull(d.Reason);
    }

    [Fact]
    public void Does_not_emit_invented_wire_kinds()
    {
        // Alias source is documented; typed-helper only for schema-accepted kinds.
        var q = Router.Route("agent-grid://server/one/question/q1");
        Assert.Equal("alias", q.Source);

        var s = Router.Route("agent-grid://server/one/session/s1");
        Assert.Equal("typed-helper", s.Source);
    }
}
