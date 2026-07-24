using System.Net.Http.Headers;
using System.Net.Http.Json;
using System.Text.Json;

namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Boundary-2 adapter: UNC-fresh token, REST ws-ticket mint, health probe for DaemonSupervisor.
/// Reuses the ticket flow (contract-b2-native-ws-auth-path) — zero server change.
/// First consumer of clients/sdk/csharp transport patterns.
/// </summary>
public sealed class ShellGatewayClient : IDisposable
{
    private readonly Uri _baseUri;
    private readonly ITokenSource _tokens;
    private readonly Func<HttpClient> _httpFactory;
    private readonly bool _ownsDefaultHttp;
    private HttpClient? _http;
    private string? _clientInstanceId;

    public ShellGatewayClient(
        Uri baseUri,
        ITokenSource tokens,
        HttpClient? http = null,
        Func<HttpClient>? httpFactory = null)
    {
        _baseUri = baseUri ?? throw new ArgumentNullException(nameof(baseUri));
        _tokens = tokens ?? throw new ArgumentNullException(nameof(tokens));
        if (httpFactory is not null)
        {
            _httpFactory = httpFactory;
            _ownsDefaultHttp = false;
        }
        else if (http is not null)
        {
            _http = http;
            _httpFactory = () => http;
            _ownsDefaultHttp = false;
        }
        else
        {
            _http = new HttpClient { BaseAddress = baseUri };
            _httpFactory = () => _http!;
            _ownsDefaultHttp = true;
        }
    }

    public string? ClientInstanceId => _clientInstanceId;

    /// <summary>
    /// Authenticated health probe used by DaemonSupervisor (GET /api/sessions).
    /// Reads a fresh token every attempt; returns false on auth/IO failure (never Connected fake).
    /// </summary>
    public async Task<bool> ProbeSessionsAsync(CancellationToken ct = default)
    {
        try
        {
            using var req = new HttpRequestMessage(HttpMethod.Get, Combine("/api/sessions"));
            await ApplyAuthAsync(req, ct).ConfigureAwait(false);
            using var res = await SendAsync(req, ct).ConfigureAwait(false);
            return res.IsSuccessStatusCode;
        }
        catch (TokenUnavailableException)
        {
            return false;
        }
        catch (HttpRequestException)
        {
            return false;
        }
        catch (TaskCanceledException) when (!ct.IsCancellationRequested)
        {
            return false;
        }
    }

    /// <summary>
    /// Two-step native WS auth: POST /api/ws-ticket → ticket + client_instance_id (FR-B2-02).
    /// Against run-dev (-no-auth) ticket mint still works; empty bearer is omitted.
    /// </summary>
    public async Task<(string Ticket, string ClientInstanceId)> MintWsTicketAsync(
        CancellationToken ct = default)
    {
        using var req = new HttpRequestMessage(HttpMethod.Post, Combine("/api/ws-ticket"));
        await ApplyAuthAsync(req, ct).ConfigureAwait(false);
        using var res = await SendAsync(req, ct).ConfigureAwait(false);
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

    /// <summary>
    /// Submit approval decision. Caller (SupervisionState service) owns optimistic rollback.
    /// </summary>
    public async Task<ApprovalSubmitResult> RespondApprovalAsync(
        string sessionId,
        string approvalId,
        string decision,
        CancellationToken ct = default)
    {
        if (decision is not ("accept" or "deny"))
            throw new ArgumentException("decision must be accept or deny", nameof(decision));

        try
        {
            using var req = new HttpRequestMessage(
                HttpMethod.Post,
                Combine($"/api/sessions/{Uri.EscapeDataString(sessionId)}/approvals/{Uri.EscapeDataString(approvalId)}"));
            await ApplyAuthAsync(req, ct).ConfigureAwait(false);
            if (_clientInstanceId is not null)
                req.Headers.TryAddWithoutValidation("X-Client-Instance-ID", _clientInstanceId);
            req.Content = JsonContent.Create(new
            {
                decision,
                client_instance_id = _clientInstanceId,
            });
            using var res = await SendAsync(req, ct).ConfigureAwait(false);
            if (res.StatusCode == System.Net.HttpStatusCode.Conflict)
                return ApprovalSubmitResult.ResolvedByOther;
            if (!res.IsSuccessStatusCode)
                return ApprovalSubmitResult.ServerError;
            return ApprovalSubmitResult.Accepted;
        }
        catch (TokenUnavailableException)
        {
            return ApprovalSubmitResult.NetworkError;
        }
        catch (HttpRequestException)
        {
            return ApprovalSubmitResult.NetworkError;
        }
        catch (TaskCanceledException) when (!ct.IsCancellationRequested)
        {
            return ApprovalSubmitResult.NetworkError;
        }
    }

    /// <summary>
    /// Submit free-text question answer (POST .../questions/{id}).
    /// </summary>
    public async Task<ApprovalSubmitResult> RespondQuestionAsync(
        string sessionId,
        string questionId,
        string answer,
        CancellationToken ct = default)
    {
        if (string.IsNullOrWhiteSpace(answer))
            throw new ArgumentException("answer required", nameof(answer));

        try
        {
            using var req = new HttpRequestMessage(
                HttpMethod.Post,
                Combine($"/api/sessions/{Uri.EscapeDataString(sessionId)}/questions/{Uri.EscapeDataString(questionId)}"));
            await ApplyAuthAsync(req, ct).ConfigureAwait(false);
            if (_clientInstanceId is not null)
                req.Headers.TryAddWithoutValidation("X-Client-Instance-ID", _clientInstanceId);
            req.Content = JsonContent.Create(new
            {
                answer,
                client_instance_id = _clientInstanceId,
            });
            using var res = await SendAsync(req, ct).ConfigureAwait(false);
            if (res.StatusCode == System.Net.HttpStatusCode.Conflict)
                return ApprovalSubmitResult.ResolvedByOther;
            if (!res.IsSuccessStatusCode)
                return ApprovalSubmitResult.ServerError;
            return ApprovalSubmitResult.Accepted;
        }
        catch (TokenUnavailableException)
        {
            return ApprovalSubmitResult.NetworkError;
        }
        catch (HttpRequestException)
        {
            return ApprovalSubmitResult.NetworkError;
        }
        catch (TaskCanceledException) when (!ct.IsCancellationRequested)
        {
            return ApprovalSubmitResult.NetworkError;
        }
    }

    public Uri WebSocketUri(string ticket) =>
        new UriBuilder(_baseUri)
        {
            Scheme = _baseUri.Scheme == "https" ? "wss" : "ws",
            Path = "/ws",
            Query = string.IsNullOrEmpty(ticket) ? "" : $"ticket={Uri.EscapeDataString(ticket)}",
        }.Uri;

    private Uri Combine(string path) => new(_baseUri, path);

    /// <summary>Omit Authorization when token is empty (run-dev -no-auth).</summary>
    private async Task ApplyAuthAsync(HttpRequestMessage req, CancellationToken ct)
    {
        var token = await _tokens.ReadFreshAsync(ct).ConfigureAwait(false);
        if (!string.IsNullOrEmpty(token))
            req.Headers.Authorization = new AuthenticationHeaderValue("Bearer", token);
    }

    private Task<HttpResponseMessage> SendAsync(HttpRequestMessage req, CancellationToken ct)
    {
        var http = _http ?? _httpFactory();
        return http.SendAsync(req, ct);
    }

    public void Dispose()
    {
        if (_ownsDefaultHttp)
            _http?.Dispose();
    }
}

public enum ApprovalSubmitResult
{
    Accepted,
    ResolvedByOther,
    NetworkError,
    ServerError,
}

/// <summary>Adapter so DaemonSupervisor can probe via ShellGatewayClient.</summary>
public sealed class GatewayHealthProbe : DaemonSupervisor.IDaemonHealthProbe
{
    private readonly ShellGatewayClient _client;

    public GatewayHealthProbe(ShellGatewayClient client) => _client = client;

    public Task<bool> ProbeAsync(CancellationToken ct = default) => _client.ProbeSessionsAsync(ct);
}
