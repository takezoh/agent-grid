using AgentGrid.Shell.Core.GatewayClient;

namespace AgentGrid.Shell.Core.Tests.GatewayClient;

public class TokenSourceTests
{
    [Fact]
    public async Task Reads_fresh_each_call()
    {
        var path = Path.Combine(Path.GetTempPath(), $"ag-token-{Guid.NewGuid():N}");
        await File.WriteAllTextAsync(path, "token-v1");
        try
        {
            var src = new FileTokenSource(path);
            Assert.Equal("token-v1", await src.ReadFreshAsync());

            await File.WriteAllTextAsync(path, "token-v2");
            Assert.Equal("token-v2", await src.ReadFreshAsync());
        }
        finally
        {
            File.Delete(path);
        }
    }

    [Fact]
    public async Task Missing_file_throws_explicit()
    {
        var path = Path.Combine(Path.GetTempPath(), $"ag-token-missing-{Guid.NewGuid():N}");
        var src = new FileTokenSource(path);
        var ex = await Assert.ThrowsAsync<TokenUnavailableException>(() => src.ReadFreshAsync());
        Assert.Equal(path, ex.Path);
    }

    [Fact]
    public async Task Empty_file_throws_explicit()
    {
        var path = Path.Combine(Path.GetTempPath(), $"ag-token-empty-{Guid.NewGuid():N}");
        await File.WriteAllTextAsync(path, "   \n");
        try
        {
            var src = new FileTokenSource(path);
            await Assert.ThrowsAsync<TokenUnavailableException>(() => src.ReadFreshAsync());
        }
        finally
        {
            File.Delete(path);
        }
    }
}
