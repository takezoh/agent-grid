using System.Net.WebSockets;
using System.Text;

namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Production ClientWebSocket adapter (Windows Shell runtime).
/// </summary>
public sealed class ClientWebSocketTransport : IWebSocketTransport
{
    private readonly ClientWebSocket _ws = new();
    private readonly byte[] _buffer = new byte[64 * 1024];

    public async Task ConnectAsync(Uri uri, CancellationToken ct = default)
    {
        await _ws.ConnectAsync(uri, ct).ConfigureAwait(false);
    }

    public async Task<string?> ReceiveTextAsync(CancellationToken ct = default)
    {
        using var ms = new MemoryStream();
        while (true)
        {
            var result = await _ws.ReceiveAsync(_buffer, ct).ConfigureAwait(false);
            if (result.MessageType == WebSocketMessageType.Close)
                return null;
            ms.Write(_buffer, 0, result.Count);
            if (result.EndOfMessage)
                break;
        }

        return Encoding.UTF8.GetString(ms.ToArray());
    }

    public async Task CloseAsync(CancellationToken ct = default)
    {
        if (_ws.State is WebSocketState.Open or WebSocketState.CloseReceived)
        {
            try
            {
                await _ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "bye", ct)
                    .ConfigureAwait(false);
            }
            catch (WebSocketException)
            {
                /* already closing */
            }
        }
    }

    public async ValueTask DisposeAsync()
    {
        try
        {
            await CloseAsync().ConfigureAwait(false);
        }
        catch
        {
            /* best-effort */
        }
        _ws.Dispose();
    }
}

public sealed class ClientWebSocketTransportFactory : IWebSocketTransportFactory
{
    public IWebSocketTransport Create() => new ClientWebSocketTransport();
}
