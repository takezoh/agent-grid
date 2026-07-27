namespace AgentGrid.Shell.Core.GatewayClient;

public static class ProtocolVersion
{
    // Projection of protocol/protocol-version.json.
    public const string Current = "protocol-2";
    public const string Header = "X-Agent-Grid-Protocol-Version";

    public static string MismatchReason(string? actual) =>
        $"incompatible protocol: client {Current}, server {actual ?? "<missing>"}";
}

public sealed class ProtocolVersionMismatchException(string? actual)
    : Exception(ProtocolVersion.MismatchReason(actual));
