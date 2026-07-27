using AgentGrid.Shell.Core.GatewayClient;

namespace AgentGrid.Shell.Core.Tests.GatewayClient;

public class TokenSourceTests
{
    [Fact]
    public async Task Returns_persisted_bearer_value()
    {
        var src = new PersistedTokenSource("token-v1");
        Assert.Equal("token-v1", await src.ReadFreshAsync());
    }

    [Fact]
    public void Empty_bearer_value_is_rejected()
    {
        Assert.Throws<ArgumentException>(() => new PersistedTokenSource("  "));
    }

    [Fact]
    public async Task No_auth_returns_empty_value()
    {
        var src = new NoAuthTokenSource();
        Assert.Equal(string.Empty, await src.ReadFreshAsync());
    }
}
