namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Fresh-read token acquisition (contract-b2-token-acquisition, FR-B2-01).
/// Never caches the bearer token across connection attempts.
/// </summary>
public interface ITokenSource
{
    /// <summary>
    /// Read the current bearer token. Throws <see cref="TokenUnavailableException"/>
    /// when the UNC path (or configured source) is unreadable (FR-B2-03).
    /// </summary>
    Task<string> ReadFreshAsync(CancellationToken ct = default);
}

public sealed class TokenUnavailableException : Exception
{
    public TokenUnavailableException(string path, Exception? inner = null)
        : base($"gateway token unreadable at '{path}'", inner)
    {
        Path = path;
    }

    public string Path { get; }
}

/// <summary>
/// File-backed token source. On Windows the path is typically
/// \\wsl$\distro\home\user\.agent-grid\gateway-token; tests inject any readable path.
/// </summary>
public sealed class FileTokenSource : ITokenSource
{
    private readonly string _path;

    public FileTokenSource(string path)
    {
        _path = path ?? throw new ArgumentNullException(nameof(path));
    }

    public async Task<string> ReadFreshAsync(CancellationToken ct = default)
    {
        try
        {
            // Fresh read every call — no static/instance cache of content.
            var text = await File.ReadAllTextAsync(_path, ct).ConfigureAwait(false);
            var token = text.Trim();
            if (string.IsNullOrEmpty(token))
                throw new TokenUnavailableException(_path);
            return token;
        }
        catch (TokenUnavailableException)
        {
            throw;
        }
        catch (Exception ex) when (ex is IOException or UnauthorizedAccessException or DirectoryNotFoundException)
        {
            throw new TokenUnavailableException(_path, ex);
        }
    }
}

/// <summary>
/// Loopback e2e against <c>scripts/run-dev.sh</c> (<c>-no-auth</c>): no bearer file.
/// Production MUST use <see cref="FileTokenSource"/>.
/// </summary>
public sealed class NoAuthTokenSource : ITokenSource
{
    public Task<string> ReadFreshAsync(CancellationToken ct = default) =>
        Task.FromResult(string.Empty);
}
