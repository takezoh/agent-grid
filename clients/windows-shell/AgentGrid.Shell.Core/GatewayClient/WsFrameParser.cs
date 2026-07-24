using System.Text.Json;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Pure parser: daemon WS text frames → SupervisionEvent.
/// Coexists with viewUpdate (k=v) and approval/question frames (k=ar|ax|qr|qx)
/// per adr-20260724-approval-broadcast-coexists-view-update.
/// </summary>
public static class WsFrameParser
{
    /// <summary>
    /// Parse one text frame. Returns null for frames the supervision surface ignores
    /// (asciicast output arrays, hello, control, unknown).
    /// </summary>
    public static SupervisionEvent? TryParse(string json)
    {
        if (string.IsNullOrWhiteSpace(json))
            return null;

        json = json.TrimStart();
        // Asciicast-style output is a JSON array — not a supervision frame.
        if (json.StartsWith('['))
            return null;

        JsonDocument doc;
        try
        {
            doc = JsonDocument.Parse(json);
        }
        catch (JsonException)
        {
            return null;
        }

        using (doc)
        {
            if (doc.RootElement.ValueKind != JsonValueKind.Object)
                return null;
            if (!doc.RootElement.TryGetProperty("k", out var kEl) ||
                kEl.ValueKind != JsonValueKind.String)
                return null;

            return kEl.GetString() switch
            {
                "ar" => ParseApprovalRequested(doc.RootElement),
                "ax" => ParseApprovalResolved(doc.RootElement),
                "qr" => ParseQuestionRequested(doc.RootElement),
                "qx" => ParseQuestionResolved(doc.RootElement),
                "v" => ParseViewUpdate(doc.RootElement),
                _ => null,
            };
        }
    }

    private static SupervisionEvent? ParseApprovalRequested(JsonElement root)
    {
        if (!root.TryGetProperty("approval", out var a))
            return null;
        var id = ReqString(a, "id");
        var sessionId = ReqString(a, "session_id");
        if (id is null || sessionId is null)
            return null;
        var summary = BuildApprovalSummary(a);
        var expires = OptDate(a, "expires_at");
        return new EvtApprovalRequested(id, sessionId, summary, expires);
    }

    private static SupervisionEvent? ParseApprovalResolved(JsonElement root)
    {
        if (!root.TryGetProperty("approval", out var a))
            return null;
        var id = ReqString(a, "id");
        var sessionId = ReqString(a, "session_id");
        if (id is null || sessionId is null)
            return null;
        var decision = OptString(a, "decision") ?? "unknown";
        var by = OptString(a, "resolving_client_instance_id");
        return new EvtApprovalResolved(id, sessionId, decision, by);
    }

    private static SupervisionEvent? ParseQuestionRequested(JsonElement root)
    {
        if (!root.TryGetProperty("question", out var q))
            return null;
        var id = ReqString(q, "id");
        var sessionId = ReqString(q, "session_id");
        if (id is null || sessionId is null)
            return null;
        var prompt = OptString(q, "prompt") ?? "";
        var expires = OptDate(q, "expires_at");
        return new EvtQuestionRequested(id, sessionId, prompt, expires);
    }

    private static SupervisionEvent? ParseQuestionResolved(JsonElement root)
    {
        if (!root.TryGetProperty("question", out var q))
            return null;
        var id = ReqString(q, "id");
        var sessionId = ReqString(q, "session_id");
        if (id is null || sessionId is null)
            return null;
        var by = OptString(q, "resolving_client_instance_id");
        return new EvtQuestionResolved(id, sessionId, by);
    }

    private static SupervisionEvent? ParseViewUpdate(JsonElement root)
    {
        if (!root.TryGetProperty("sessions", out var sessionsEl) ||
            sessionsEl.ValueKind != JsonValueKind.Array)
            return null;

        var list = new List<SessionSummary>();
        foreach (var s in sessionsEl.EnumerateArray())
        {
            var id = ReqString(s, "id");
            if (id is null)
                continue;
            var phase = MapPhase(OptString(s, "state"));
            string? title = null;
            if (s.TryGetProperty("view", out var view) &&
                view.ValueKind == JsonValueKind.Object &&
                view.TryGetProperty("card", out var card) &&
                card.ValueKind == JsonValueKind.Object)
            {
                title = OptString(card, "title");
            }
            title ??= OptString(s, "project") ?? OptString(s, "command");
            list.Add(new SessionSummary(id, phase, title));
        }

        return new EvtViewUpdateSessions(list);
    }

    private static string BuildApprovalSummary(JsonElement a)
    {
        var command = OptString(a, "command");
        if (!string.IsNullOrEmpty(command))
            return command!;
        var path = OptString(a, "path");
        if (!string.IsNullOrEmpty(path))
            return path!;
        var reason = OptString(a, "reason");
        if (!string.IsNullOrEmpty(reason))
            return reason!;
        return OptString(a, "kind") ?? "approval";
    }

    private static SessionPhase MapPhase(string? state) =>
        state?.ToLowerInvariant() switch
        {
            "running" or "thinking" or "streaming" => SessionPhase.Running,
            "waiting" or "pending" or "approval" or "question" => SessionPhase.Waiting,
            "failed" or "error" => SessionPhase.Failed,
            "done" or "stopped" or "idle" => SessionPhase.Done,
            _ => SessionPhase.Running,
        };

    private static string? ReqString(JsonElement el, string name) =>
        el.TryGetProperty(name, out var p) && p.ValueKind == JsonValueKind.String
            ? p.GetString()
            : null;

    private static string? OptString(JsonElement el, string name) => ReqString(el, name);

    private static DateTimeOffset? OptDate(JsonElement el, string name)
    {
        var s = OptString(el, name);
        if (s is null)
            return null;
        return DateTimeOffset.TryParse(s, out var dto) ? dto : null;
    }
}
