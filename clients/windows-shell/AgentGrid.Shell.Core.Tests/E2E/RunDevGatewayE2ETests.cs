using System.Net.WebSockets;
using System.Text;
using AgentGrid.Shell.Core.GatewayClient;

namespace AgentGrid.Shell.Core.Tests.E2E;

/// <summary>
/// T3 fidelity: Shell.Core against real run-dev gateway (no Windows server spawn).
/// Always-on CI leaves these skipped unless AG_E2E_RUN_DEV=1.
/// </summary>
public class RunDevGatewayE2ETests
{
    [RunDevFact]
    public async Task Probe_sessions_against_run_dev_no_auth()
    {
        await RunDevGateway.EnsureReachableAsync();

        var gateway = new ShellGatewayClient(
            RunDevGateway.BaseUri,
            new NoAuthTokenSource());
        Assert.True(await gateway.ProbeSessionsAsync());
    }

    [RunDevFact]
    public async Task Mint_ticket_and_open_websocket()
    {
        await RunDevGateway.EnsureReachableAsync();

        var gateway = new ShellGatewayClient(
            RunDevGateway.BaseUri,
            new NoAuthTokenSource());
        var (ticket, clientInstanceId) = await gateway.MintWsTicketAsync();
        Assert.False(string.IsNullOrEmpty(ticket));
        Assert.False(string.IsNullOrEmpty(clientInstanceId));

        using var ws = new ClientWebSocket();
        using var cts = new CancellationTokenSource(TimeSpan.FromSeconds(10));
        await ws.ConnectAsync(gateway.WebSocketUri(ticket), cts.Token);
        Assert.Equal(WebSocketState.Open, ws.State);

        var buf = new byte[4096];
        try
        {
            using var readCts = new CancellationTokenSource(TimeSpan.FromSeconds(3));
            var result = await ws.ReceiveAsync(buf, readCts.Token);
            Assert.True(result.Count >= 0);
            if (result.MessageType == WebSocketMessageType.Text)
            {
                var text = Encoding.UTF8.GetString(buf, 0, result.Count);
                Assert.False(string.IsNullOrWhiteSpace(text));
            }
        }
        catch (OperationCanceledException)
        {
            Assert.Equal(WebSocketState.Open, ws.State);
        }

        await ws.CloseAsync(WebSocketCloseStatus.NormalClosure, "e2e", CancellationToken.None);
    }

}
