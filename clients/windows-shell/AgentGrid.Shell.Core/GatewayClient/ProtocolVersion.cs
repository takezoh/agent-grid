namespace AgentGrid.Shell.Core.GatewayClient;

public static class ProtocolVersion
{
    // Projection of protocol/protocol-version.json.
    public const string Current = "protocol-3";
    public const string Header = "X-Agent-Grid-Protocol-Version";

    public static string MismatchReason(string? actual) =>
        $"incompatible protocol: client {Current}, server {actual ?? "<missing>"}";

    /// <summary>Exact-version gate: side effects must not run on mismatch.</summary>
    public static void AdmitRepositoryOperation(string? actual, Action? sideEffect = null)
    {
        if (!string.Equals(actual, Current, StringComparison.Ordinal))
            throw new ProtocolVersionMismatchException(actual);
        sideEffect?.Invoke();
    }
}

public sealed class ProtocolVersionMismatchException(string? actual)
    : Exception(ProtocolVersion.MismatchReason(actual));

public sealed record GitHubRepositoryIdentity(string RepositoryId, string Owner, string Name);
public sealed record SessionTarget(string Source, string? ServerId = null, string? Project = null, GitHubRepositoryIdentity? Repository = null);
public sealed record RepositoryPreparationFrame(
    string PreparationId, string IntentId, string ServerId, GitHubRepositoryIdentity Repository,
    string Stage, ulong Revision, string? FailureCode = null, string? RetryOrigin = null,
    string? EnvironmentDisposition = null, string? SessionId = null, string? ClonePath = null, string? EnvironmentId = null);
