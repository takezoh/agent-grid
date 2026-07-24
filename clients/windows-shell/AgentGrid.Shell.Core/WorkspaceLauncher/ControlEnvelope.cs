using System.Text.Json;
using System.Text.Json.Serialization;

namespace AgentGrid.Shell.Core.WorkspaceLauncher;

/// <summary>
/// Closed JSON Lines envelope for Boundary-1.
/// (contract-b1-jsonlines-envelope-shape, adr-20260724-boundary-1-named-pipe-jsonlines).
/// additionalProperties: false — any extra field is rejected.
/// </summary>
public sealed class ControlEnvelope
{
    public const int CurrentSchemaVersion = 2;

    [JsonPropertyName("op")]
    public required string Op { get; init; }

    [JsonPropertyName("server_id")]
    public string? ServerId { get; init; }

    [JsonPropertyName("session_id")]
    public string? SessionId { get; init; }

    [JsonPropertyName("schema_version")]
    public int SchemaVersion { get; init; } = CurrentSchemaVersion;

    public static readonly HashSet<string> AllowedOps = new(StringComparer.Ordinal)
    {
        "openSession",
        "activate",
        "quit",
    };

    private static readonly JsonSerializerOptions StrictOptions = new()
    {
        PropertyNameCaseInsensitive = false,
        ReadCommentHandling = JsonCommentHandling.Disallow,
        AllowTrailingCommas = false,
        // We enforce closed schema manually after deserialize.
    };

    public static EnvelopeParseResult ParseLine(string line)
    {
        if (string.IsNullOrWhiteSpace(line))
            return EnvelopeParseResult.Fail("empty line");

        JsonDocument doc;
        try
        {
            doc = JsonDocument.Parse(line);
        }
        catch (JsonException ex)
        {
            return EnvelopeParseResult.Fail($"malformed json: {ex.Message}");
        }

        using (doc)
        {
            if (doc.RootElement.ValueKind != JsonValueKind.Object)
                return EnvelopeParseResult.Fail("envelope must be an object");

            // Closed schema: only op, compound session identity, schema_version.
            foreach (var prop in doc.RootElement.EnumerateObject())
            {
                if (prop.Name is not ("op" or "server_id" or "session_id" or "schema_version"))
                    return EnvelopeParseResult.Fail($"unknown field: {prop.Name}");
            }

            if (!doc.RootElement.TryGetProperty("op", out var opEl) ||
                opEl.ValueKind != JsonValueKind.String)
                return EnvelopeParseResult.Fail("missing op");

            var op = opEl.GetString()!;
            if (!AllowedOps.Contains(op))
                return EnvelopeParseResult.Fail($"unknown op: {op}");

            string? serverId = null;
            if (doc.RootElement.TryGetProperty("server_id", out var serverEl))
            {
                if (serverEl.ValueKind != JsonValueKind.String)
                    return EnvelopeParseResult.Fail("server_id must be a string");
                serverId = serverEl.GetString();
            }

            string? sessionId = null;
            if (doc.RootElement.TryGetProperty("session_id", out var sessionEl))
            {
                if (sessionEl.ValueKind != JsonValueKind.String)
                    return EnvelopeParseResult.Fail("session_id must be a string");
                sessionId = sessionEl.GetString();
            }

            if (op == "openSession" &&
                (string.IsNullOrEmpty(serverId) || string.IsNullOrEmpty(sessionId)))
                return EnvelopeParseResult.Fail("openSession requires server_id and session_id");

            var schemaVersion = CurrentSchemaVersion;
            if (doc.RootElement.TryGetProperty("schema_version", out var svEl))
            {
                if (svEl.ValueKind != JsonValueKind.Number || !svEl.TryGetInt32(out schemaVersion))
                    return EnvelopeParseResult.Fail("schema_version must be int");
            }

            return EnvelopeParseResult.Ok(new ControlEnvelope
            {
                Op = op,
                ServerId = serverId,
                SessionId = sessionId,
                SchemaVersion = schemaVersion,
            });
        }
    }

    public string ToJsonLine()
    {
        var obj = new Dictionary<string, object?>
        {
            ["op"] = Op,
            ["schema_version"] = SchemaVersion,
        };
        if (ServerId is not null)
            obj["server_id"] = ServerId;
        if (SessionId is not null)
            obj["session_id"] = SessionId;
        return JsonSerializer.Serialize(obj, StrictOptions);
    }
}

public sealed class EnvelopeParseResult
{
    public bool Success { get; init; }
    public ControlEnvelope? Envelope { get; init; }
    public string? Error { get; init; }

    public static EnvelopeParseResult Ok(ControlEnvelope env) =>
        new() { Success = true, Envelope = env };

    public static EnvelopeParseResult Fail(string message) =>
        new() { Success = false, Error = message };
}

public sealed class ControlReply
{
    [JsonPropertyName("ok")]
    public bool? Ok { get; init; }

    [JsonPropertyName("error")]
    public string? Error { get; init; }

    public static ControlReply Success() => new() { Ok = true };
    public static ControlReply Fail(string error) => new() { Ok = false, Error = error };

    public string ToJsonLine()
    {
        if (Error is not null)
            return JsonSerializer.Serialize(new { ok = false, error = Error });
        return JsonSerializer.Serialize(new { ok = true });
    }
}
