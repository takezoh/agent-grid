namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Supplies the validated credential selected by desktop configuration.
/// </summary>
public interface ITokenSource
{
    /// <summary>
    /// Return the configured bearer token, or empty for explicit no-auth mode.
    /// </summary>
    Task<string> ReadFreshAsync(CancellationToken ct = default);
}

public sealed class PersistedTokenSource : ITokenSource
{
    private readonly string _token;

    public PersistedTokenSource(string token)
    {
        if (string.IsNullOrWhiteSpace(token))
            throw new ArgumentException("Bearer token must not be empty.", nameof(token));
        _token = token;
    }

    public Task<string> ReadFreshAsync(CancellationToken ct = default) =>
        Task.FromResult(_token);
}

/// <summary>
/// Explicit no-auth configuration. The gateway client omits Authorization.
/// </summary>
public sealed class NoAuthTokenSource : ITokenSource
{
    public Task<string> ReadFreshAsync(CancellationToken ct = default) =>
        Task.FromResult(string.Empty);
}
