namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Injectable WS transport for SupervisionWsSession (fakeable in xUnit).
/// Production uses ClientWebSocketTransport.
/// </summary>
public interface IWebSocketTransport : IAsyncDisposable
{
    Task ConnectAsync(Uri uri, CancellationToken ct = default);

    /// <summary>Receive one text frame; null means clean close.</summary>
    Task<string?> ReceiveTextAsync(CancellationToken ct = default);

    Task CloseAsync(CancellationToken ct = default);
}

public interface IWebSocketTransportFactory
{
    IWebSocketTransport Create();
}
