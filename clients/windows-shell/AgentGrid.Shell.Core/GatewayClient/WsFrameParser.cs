using System.Text.Json;
using AgentGrid.Shell.Core.SupervisionState;

namespace AgentGrid.Shell.Core.GatewayClient;

/// <summary>
/// Pure parser: daemon WS text frames → SupervisionEvent.
/// Coexists with viewUpdate (k=v) and approval/question frames (k=ar|ax|qr|qx)
/// per adr-20260724-approval-broadcast-coexists-view-update.
///
/// Hello/delta contract (services/server/api/gateway.go helloFrame /
/// lifecycleFrames): the gateway sends the full current session list exactly
/// once per lifecycle WS connection, as a k="h" hello frame, synchronously in
/// reaction to subscribing. Every session-list change after that point is
/// sent as a k="v" delta frame instead — "h" never recurs on the same
/// connection. Both carry the sessions array in the same shape, so both are
/// routed through ParseViewUpdate here: a client that only handles "v" and
/// connects after sessions already exist would otherwise never learn about
/// them, since no further "v" frame is guaranteed to ever arrive.
/// </summary>
public static class WsFrameParser
{
    private static readonly object SignalLock = new();
    private static readonly Dictionary<string, long> HighestConsumedSignal = new();

    /// <summary>
    /// Parse one text frame. Returns null for frames the supervision surface ignores
    /// (asciicast output arrays, control, unknown).
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
                "h" => ParseHello(doc.RootElement),
                "v" => ParseViewUpdate(doc.RootElement),
                _ => null,
            };
        }
    }

    /// <summary>Parse preparation snapshots; protocol mismatch rejects before side effects.</summary>
    public static IReadOnlyList<RepositoryPreparationFrame> TryParsePreparations(string json)
    {
        if (string.IsNullOrWhiteSpace(json)) return Array.Empty<RepositoryPreparationFrame>();
        using var doc = JsonDocument.Parse(json);
        var root = doc.RootElement;
        if (root.ValueKind != JsonValueKind.Object) return Array.Empty<RepositoryPreparationFrame>();
        if (root.TryGetProperty("protocolVersion", out var pv) && pv.ValueKind == JsonValueKind.String)
            ProtocolVersion.AdmitRepositoryOperation(pv.GetString());
        else if (OptString(root, "k") is "h" or "rp")
            ProtocolVersion.AdmitRepositoryOperation(null);

        var list = new List<RepositoryPreparationFrame>();
        if (root.TryGetProperty("preparations", out var preps) && preps.ValueKind == JsonValueKind.Array)
        {
            foreach (var el in preps.EnumerateArray())
                if (ParsePreparationElement(el) is { } p) list.Add(p);
        }
        else if (OptString(root, "k") == "rp" && root.TryGetProperty("preparation", out var one) &&
                 ParsePreparationElement(one) is { } single)
        {
            list.Add(single);
        }
        return list;
    }

    public static SessionTarget? TryParseSessionTarget(string json)
    {
        if (string.IsNullOrWhiteSpace(json)) return null;
        using var doc = JsonDocument.Parse(json);
        var root = doc.RootElement;
        var source = OptString(root, "source");
        if (source == "project" && root.TryGetProperty("project", out var p) && p.ValueKind == JsonValueKind.Object)
        {
            var sid = OptString(p, "server_id"); var proj = OptString(p, "project");
            return string.IsNullOrEmpty(sid) || string.IsNullOrEmpty(proj) ? null
                : new SessionTarget("project", ServerId: sid, Project: proj);
        }
        if (source == "repository" && root.TryGetProperty("repository", out var r) && r.ValueKind == JsonValueKind.Object)
        {
            var id = OptString(r, "repository_id"); var owner = OptString(r, "owner"); var name = OptString(r, "name");
            return string.IsNullOrEmpty(id) || string.IsNullOrEmpty(owner) || string.IsNullOrEmpty(name) ? null
                : new SessionTarget("repository", Repository: new GitHubRepositoryIdentity(id!, owner!, name!));
        }
        return null;
    }

    private static RepositoryPreparationFrame? ParsePreparationElement(JsonElement el)
    {
        if (el.ValueKind != JsonValueKind.Object) return null;
        var prepId = OptString(el, "preparation_id"); var intentId = OptString(el, "intent_id");
        var serverId = OptString(el, "server_id"); var stage = OptString(el, "stage");
        if (prepId is null || intentId is null || serverId is null || stage is null) return null;
        if (!el.TryGetProperty("repository", out var repo) || repo.ValueKind != JsonValueKind.Object) return null;
        var rid = OptString(repo, "repository_id"); var owner = OptString(repo, "owner"); var name = OptString(repo, "name");
        if (rid is null || owner is null || name is null) return null;
        ulong revision = el.TryGetProperty("revision", out var revEl) && revEl.TryGetUInt64(out var rev) ? rev : 0;
        var sessionId = OptString(el, "session_id");
        if (stage == "available" && string.IsNullOrEmpty(sessionId)) return null;
        if (stage != "available" && !string.IsNullOrEmpty(sessionId)) return null;
        return new RepositoryPreparationFrame(prepId, intentId, serverId,
            new GitHubRepositoryIdentity(rid, owner, name), stage, revision,
            OptString(el, "failure_code"), OptString(el, "retry_origin"),
            OptString(el, "environment_disposition"), sessionId,
            OptString(el, "clone_path"), OptString(el, "environment_id"));
    }

    private static SupervisionEvent? ParseHello(JsonElement root)
    {
        var actual = OptString(root, "protocolVersion");
        if (actual != ProtocolVersion.Current)
            throw new ProtocolVersionMismatchException(actual);
        return ParseViewUpdate(root);
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
            if (!s.TryGetProperty("status", out var status) ||
                status.ValueKind != JsonValueKind.Object)
                continue;
            var lifecycle = OptString(status, "lifecycle");
            var failure = OptString(status, "failure");
            if (lifecycle is not ("active" or "idle" or "stopped") ||
                failure is not ("none" or "failed") ||
                !status.TryGetProperty("human_input", out var input) ||
                input.ValueKind is not (JsonValueKind.True or JsonValueKind.False))
                continue;
            var phase = MapPhase(lifecycle, failure);
            var humanInput = input.ValueKind == JsonValueKind.True;
            var bell = ConsumeBell(id, status);
            string? title = null;
            if (s.TryGetProperty("view", out var view) &&
                view.ValueKind == JsonValueKind.Object &&
                view.TryGetProperty("card", out var card) &&
                card.ValueKind == JsonValueKind.Object)
            {
                title = OptString(card, "title");
            }
            list.Add(new SessionSummary(
                id, phase, title, HumanInput: humanInput, Bell: bell, Lifecycle: lifecycle));
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

    private static SessionPhase MapPhase(string? lifecycle, string? failure) =>
        failure == "failed" ? SessionPhase.Failed :
        lifecycle?.ToLowerInvariant() switch
        {
            "active" => SessionPhase.Running,
            "stopped" or "idle" => SessionPhase.Done,
            _ => SessionPhase.Running,
        };

    private static bool ConsumeBell(string sessionId, JsonElement status)
    {
        if (!status.TryGetProperty("signal", out var signal) ||
            signal.ValueKind != JsonValueKind.Object ||
            OptString(signal, "kind") != "bell")
            return false;
        var epoch = OptString(signal, "epoch");
        if (string.IsNullOrEmpty(epoch) ||
            !signal.TryGetProperty("sequence", out var sequenceEl) ||
            !sequenceEl.TryGetInt64(out var sequence) ||
            sequence < 1)
            return false;
        var key = sessionId + "\0" + epoch;
        lock (SignalLock)
        {
            if (HighestConsumedSignal.TryGetValue(key, out var highest) && sequence <= highest)
                return false;
            HighestConsumedSignal[key] = sequence;
            return true;
        }
    }

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
