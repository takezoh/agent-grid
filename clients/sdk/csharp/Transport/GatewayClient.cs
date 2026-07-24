// Hand-written thin REST transport for the C# SDK.
// Routes: protocol/openapi.yaml (REST-binding annex).
// Models: ../Generated (quicktype). First consumer: windows-shell GatewayClient.

using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;

namespace AgentGrid.Client.Transport;

public sealed class GatewayClient : IDisposable
{
    private readonly HttpClient _http;
    private readonly bool _ownsHttp;
    private string? _clientInstanceId;

    public GatewayClient(Uri baseUri, string? bearerToken = null, HttpClient? http = null)
    {
        _ownsHttp = http is null;
        _http = http ?? new HttpClient { BaseAddress = baseUri };
        if (bearerToken is not null)
        {
            _http.DefaultRequestHeaders.Authorization =
                new AuthenticationHeaderValue("Bearer", bearerToken);
        }
    }

    public string? ClientInstanceId => _clientInstanceId;

    public async Task<(string Ticket, string ClientInstanceId)> MintWsTicketAsync(
        CancellationToken ct = default)
    {
        using var res = await _http.PostAsync("/api/ws-ticket", content: null, ct).ConfigureAwait(false);
        res.EnsureSuccessStatusCode();
        await using var stream = await res.Content.ReadAsStreamAsync(ct).ConfigureAwait(false);
        using var doc = await JsonDocument.ParseAsync(stream, cancellationToken: ct).ConfigureAwait(false);
        var ticket = doc.RootElement.GetProperty("ticket").GetString()
            ?? throw new InvalidOperationException("missing ticket");
        var ci = doc.RootElement.GetProperty("client_instance_id").GetString()
            ?? throw new InvalidOperationException("missing client_instance_id");
        _clientInstanceId = ci;
        return (ticket, ci);
    }

    public async Task RespondApprovalAsync(
        string sessionId,
        string approvalId,
        string decision,
        CancellationToken ct = default)
    {
        if (decision is not ("accept" or "deny"))
            throw new ArgumentException("decision must be accept or deny", nameof(decision));
        using var req = new HttpRequestMessage(
            HttpMethod.Post,
            $"/api/sessions/{Uri.EscapeDataString(sessionId)}/approvals/{Uri.EscapeDataString(approvalId)}");
        if (_clientInstanceId is not null)
            req.Headers.TryAddWithoutValidation("X-Client-Instance-ID", _clientInstanceId);
        req.Content = JsonContent.Create(new
        {
            decision,
            client_instance_id = _clientInstanceId,
        });
        using var res = await _http.SendAsync(req, ct).ConfigureAwait(false);
        res.EnsureSuccessStatusCode();
    }

    public void Dispose()
    {
        if (_ownsHttp)
            _http.Dispose();
    }
}
