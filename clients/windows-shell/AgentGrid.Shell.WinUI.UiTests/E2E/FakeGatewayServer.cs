using System.Net;
using System.Net.Sockets;
using System.Net.WebSockets;
using System.Text;

namespace AgentGrid.Shell.WinUI.UiTests.E2E;

/// <summary>
/// Deterministic Shell gateway fake for native UIA tests. It owns only the
/// REST/WS contract consumed by Shell; Windows rendering remains black-box.
/// </summary>
internal sealed class FakeGatewayServer : IDisposable
{
    private static readonly string[] SessionFrames =
    [
        """
        {"k":"h","sessions":[
          {"id":"working-session","status":{"lifecycle":"active","human_input":false,"failure":"none"},"view":{"card":{"title":"Working fixture"}}},
          {"id":"action-session","status":{"lifecycle":"idle","human_input":true,"failure":"none","signal":{"kind":"bell","epoch":"ui-e2e","sequence":1}},"view":{"card":{"title":"Action fixture"}}}
        ]}
        """,
    ];

    private static readonly string[] RequestFrames =
    [
        """{"k":"ar","approval":{"id":"approval-1","session_id":"action-session","status":"pending","command":"git status"}}""",
        """{"k":"qr","question":{"id":"question-1","session_id":"action-session","status":"pending","prompt":"Continue?"}}""",
    ];

    private readonly HttpListener _listener = new();
    private readonly CancellationTokenSource _stop = new();
    private readonly Task _serveTask;

    public FakeGatewayServer(bool includeRequests)
    {
        Frames = includeRequests
            ? [.. SessionFrames, .. RequestFrames]
            : SessionFrames;
        var port = ReservePort();
        BaseUri = new Uri($"http://127.0.0.1:{port}/");
        _listener.Prefixes.Add(BaseUri.ToString());
        _listener.Start();
        _serveTask = Task.Run(() => ServeAsync(_stop.Token));
    }

    public Uri BaseUri { get; }
    private string[] Frames { get; }

    private async Task ServeAsync(CancellationToken ct)
    {
        while (!ct.IsCancellationRequested)
        {
            HttpListenerContext context;
            try
            {
                context = await _listener.GetContextAsync().WaitAsync(ct);
            }
            catch (OperationCanceledException)
            {
                return;
            }
            catch (HttpListenerException) when (ct.IsCancellationRequested)
            {
                return;
            }

            _ = Task.Run(() => HandleAsync(context, ct), ct);
        }
    }

    private async Task HandleAsync(HttpListenerContext context, CancellationToken ct)
    {
        var path = context.Request.Url?.AbsolutePath ?? "";
        if (path == "/ws" && context.Request.IsWebSocketRequest)
        {
            var upgraded = await context.AcceptWebSocketAsync(null);
            foreach (var frame in Frames)
            {
                var bytes = Encoding.UTF8.GetBytes(frame);
                await upgraded.WebSocket.SendAsync(
                    bytes,
                    WebSocketMessageType.Text,
                    endOfMessage: true,
                    ct);
            }
            try
            {
                await Task.Delay(Timeout.InfiniteTimeSpan, ct);
            }
            catch (OperationCanceledException)
            {
                // Fixture shutdown.
            }
            finally
            {
                upgraded.WebSocket.Dispose();
            }
            return;
        }

        if (path == "/api/ws-ticket")
        {
            await JsonAsync(
                context.Response,
                """{"ticket":"ui-e2e-ticket","client_instance_id":"ui-e2e-client"}""",
                ct);
            return;
        }
        if (path == "/api/sessions")
        {
            await JsonAsync(context.Response, """{"sessions":[]}""", ct);
            return;
        }
        if (path.Contains("/approvals/", StringComparison.Ordinal) ||
            path.Contains("/questions/", StringComparison.Ordinal))
        {
            await JsonAsync(context.Response, "{}", ct);
            return;
        }

        context.Response.StatusCode = (int)HttpStatusCode.NotFound;
        context.Response.Close();
    }

    private static async Task JsonAsync(
        HttpListenerResponse response,
        string json,
        CancellationToken ct)
    {
        var bytes = Encoding.UTF8.GetBytes(json);
        response.StatusCode = (int)HttpStatusCode.OK;
        response.ContentType = "application/json";
        response.ContentLength64 = bytes.Length;
        await response.OutputStream.WriteAsync(bytes, ct);
        response.Close();
    }

    private static int ReservePort()
    {
        using var socket = new TcpListener(IPAddress.Loopback, 0);
        socket.Start();
        return ((IPEndPoint)socket.LocalEndpoint).Port;
    }

    public void Dispose()
    {
        _stop.Cancel();
        _listener.Close();
        try { _serveTask.Wait(TimeSpan.FromSeconds(5)); }
        catch (AggregateException) { /* cancellation */ }
        _stop.Dispose();
    }
}
