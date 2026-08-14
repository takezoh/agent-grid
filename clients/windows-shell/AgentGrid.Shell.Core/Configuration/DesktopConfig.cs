using System.Text.Json;
using System.Text.Json.Serialization;
using System.Security.Cryptography;
using System.Text;

namespace AgentGrid.Shell.Core.Configuration;

public sealed record DesktopConfig(
    IReadOnlyList<ServerConfig> Servers,
    AppearanceConfig Appearance,
    ShellAppConfig Shell,
    WorkspaceAppConfig Workspace,
    string ConfigDirectory);

/// <summary>
/// How an entry may be used. Admission is per entry: a malformed entry costs
/// that entry and the entries it collides with, not the fleet
/// (adr-20260814-server-display-name-naming-contract).
/// </summary>
public static class AdmissionStates
{
    public const string Usable = "usable";
    public const string Disabled = "disabled";
    public const string ConfigurationInvalid = "configuration_invalid";
}

/// <summary>
/// Rule identifiers, shared with clients/config/servers.schema.json
/// x-contract-rules and with the expected-outcome corpus.
/// </summary>
public static class ServerRules
{
    public const string DisplayNameRequired = "display-name-required";
    public const string DisplayNameCharset = "display-name-charset";
    public const string DisplayNameUnique = "display-name-unique";
    public const string EntryFieldTypes = "entry-field-types";
    public const string UnknownEntryField = "unknown-entry-field-rejected";
    public const string UrlScheme = "url-http-or-https";
    public const string UrlBareAuthority = "url-carries-no-credentials-query-or-fragment";
    public const string DuplicateUrl = "duplicate-url-rejected";
    public const string AuthVariant = "auth-variant-exact";
    public const string AtMostOneDefault = "at-most-one-default-execution";
    public const string DefaultMustBeEnabled = "default-execution-must-be-enabled";
}

/// <summary>Why an entry is unusable: the rule it broke and enough detail to find it.</summary>
public sealed record ServerViolation(string Rule, string Detail);

public sealed record ServerConfig(
    string Id,
    int Index,
    string DisplayName,
    bool Enabled,
    bool DefaultExecution,
    Uri? Url,
    ServerAuthConfig Auth,
    string Admission,
    ServerViolation? Violation = null)
{
    /// <summary>An entry a client may connect to. Invalid and disabled entries are not.</summary>
    public bool IsUsable => Admission == AdmissionStates.Usable;
}

public enum ServerAuthMode
{
    None,
    Bearer,
}

public sealed record ServerAuthConfig(ServerAuthMode Mode, string? Token)
{
    public static ServerAuthConfig None { get; } = new(ServerAuthMode.None, null);
    public static ServerAuthConfig Bearer(string token) => new(ServerAuthMode.Bearer, token);
}

public sealed record AppearanceConfig(string Theme, string Density, double FontScale);

public sealed record DisplayAnchor(int X, int Y);

public sealed record ShellAppConfig(
    string WorkspaceExecutable,
    int HealthPollIntervalSeconds,
    DisplayAnchor? DisplayAnchor);

public sealed record WorkspaceAppConfig(
    int IdleQuitSeconds,
    int DefaultWindowWidth,
    int DefaultWindowHeight);

/// <summary>
/// Loads the four desktop configuration files shared by Shell and Workspace.
/// Missing files are created from the same schema-versioned defaults. Existing
/// files are never overwritten.
/// </summary>
public static class DesktopConfigLoader
{
    public const int SchemaVersion = 1;
    public const int ServersSchemaVersion = 2;
    public const string ConfigDirectoryArgument = "--config-dir";

    private static readonly JsonSerializerOptions JsonOptions = new()
    {
        PropertyNameCaseInsensitive = false,
        ReadCommentHandling = JsonCommentHandling.Disallow,
        AllowTrailingCommas = false,
        WriteIndented = true,
        UnmappedMemberHandling = JsonUnmappedMemberHandling.Disallow,
    };

    public static string DefaultConfigDirectory()
    {
        var appData = Environment.GetEnvironmentVariable("APPDATA");
        if (string.IsNullOrWhiteSpace(appData))
            appData = Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData);
        return Path.Combine(appData, "agent-grid", "config");
    }

    public static string ResolveConfigDirectory(IEnumerable<string> args)
    {
        var values = args.ToArray();
        for (var i = 0; i < values.Length; i++)
        {
            if (values[i] != ConfigDirectoryArgument)
                continue;
            if (i + 1 >= values.Length || string.IsNullOrWhiteSpace(values[i + 1]))
                throw new DesktopConfigException("--config-dir requires a path");
            return Path.GetFullPath(values[i + 1]);
        }
        return DefaultConfigDirectory();
    }

    public static DesktopConfig LoadOrCreate(string configDirectory)
    {
        configDirectory = Path.GetFullPath(configDirectory);
        Directory.CreateDirectory(configDirectory);

        var defaults = DefaultDocuments();
        foreach (var (name, value) in defaults)
            CreateIfMissing(Path.Combine(configDirectory, name), value);

        var servers = Read<ServersReadDocument>(configDirectory, "servers.json");
        var appearance = Read<AppearanceDocument>(configDirectory, "appearance.json");
        var shell = Read<ShellDocument>(configDirectory, "shell.json");
        var workspace = Read<WorkspaceDocument>(configDirectory, "workspace.json");

        ValidateVersion(servers.SchemaVersion, "servers.json", ServersSchemaVersion);
        ValidateVersion(appearance.SchemaVersion, "appearance.json");
        ValidateVersion(shell.SchemaVersion, "shell.json");
        ValidateVersion(workspace.SchemaVersion, "workspace.json");

        // An absent entry list names no entry, so it is the one remaining
        // file-level stop besides unreadable bytes and a wrong schema version.
        // An empty list is simply an empty fleet.
        if (servers.Servers is null)
            throw new DesktopConfigException("servers.json: servers must be an array");
        var mappedServers = AdmitServers(servers.Servers);

        ValidateAppearance(appearance);
        ValidateShell(shell);
        ValidateWorkspace(workspace);

        return new DesktopConfig(
            mappedServers,
            new AppearanceConfig(appearance.Theme, appearance.Density, appearance.FontScale),
            new ShellAppConfig(
                shell.WorkspaceExecutable,
                shell.HealthPollIntervalSeconds,
                shell.DisplayAnchor is null
                    ? null
                    : new DisplayAnchor(shell.DisplayAnchor.X, shell.DisplayAnchor.Y)),
            new WorkspaceAppConfig(
                workspace.IdleQuitSeconds,
                workspace.DefaultWindow.Width,
                workspace.DefaultWindow.Height),
            configDirectory);
    }

    // Admission is per entry. Entry-local rules run first so that a relation
    // between entries — a duplicate name, a duplicate URL, two default-execution
    // markers — is only computed over entries that are otherwise sound. Nothing
    // here renames, normalises, suffixes, disables or drops an entry to resolve
    // a violation (adr-20260814-server-display-name-naming-contract).
    private static List<ServerConfig> AdmitServers(JsonElement[] raw)
    {
        var servers = raw.Select(AdmitEntry).ToList();
        MarkCollisions(
            servers,
            server => string.IsNullOrEmpty(server.Id) ? null : server.Id,
            ServerRules.DuplicateUrl,
            "two entries resolve to the same canonical url");
        MarkCollisions(
            servers,
            server => server.Enabled ? DisplayNameKey(server.DisplayName) : null,
            ServerRules.DisplayNameUnique,
            "two enabled entries share a display name");
        MarkDefaultExecution(servers);
        return servers;
    }

    private static ServerConfig AdmitEntry(JsonElement raw, int index)
    {
        var blank = new ServerConfig(
            "", index, "", false, false, null, ServerAuthConfig.None,
            AdmissionStates.ConfigurationInvalid);

        if (raw.ValueKind != JsonValueKind.Object)
            return blank with
            {
                Violation = new ServerViolation(
                    ServerRules.EntryFieldTypes, "entry must be an object"),
            };
        // Checked before decoding rather than read off a decoder message, so
        // the two entry-scoped decode failures stay distinguishable: a field
        // the contract does not define, and a value of the wrong type.
        var unknown = UnknownEntryField(raw);
        if (unknown is not null)
            return blank with
            {
                DisplayName = SalvageDisplayName(raw),
                Violation = new ServerViolation(
                    ServerRules.UnknownEntryField, $"unsupported field '{unknown}'"),
            };

        ServerDocument? document;
        try
        {
            document = raw.Deserialize<ServerDocument>(JsonOptions);
        }
        catch (JsonException error)
        {
            return blank with
            {
                DisplayName = SalvageDisplayName(raw),
                Violation = new ServerViolation(ServerRules.EntryFieldTypes, error.Message),
            };
        }
        if (document is null)
            return blank with { Violation = new ServerViolation(ServerRules.EntryFieldTypes, "entry must be an object") };

        var entry = blank with
        {
            DisplayName = document.DisplayName ?? "",
            Enabled = document.Enabled,
            DefaultExecution = document.DefaultExecution == true,
        };

        var nameViolation = CheckDisplayName(entry.DisplayName);
        if (nameViolation is not null)
            return entry with { Violation = nameViolation };

        if (!Uri.TryCreate(document.Url, UriKind.Absolute, out var url) ||
            url.Scheme is not ("http" or "https"))
            return entry with
            {
                Violation = new ServerViolation(
                    ServerRules.UrlScheme, "url must be an absolute HTTP(S) URL"),
            };
        // Rejected, not stripped. All three are things an operator wrote on
        // purpose; discarding them silently leaves the operator believing the
        // client is doing something it is not — most sharply for a credential,
        // where the belief is that authentication is configured
        // (adr-20260814-shared-go-client-config-module).
        if (!string.IsNullOrEmpty(url.UserInfo) ||
            !string.IsNullOrEmpty(url.Query) ||
            !string.IsNullOrEmpty(url.Fragment))
            return entry with
            {
                Violation = new ServerViolation(
                    ServerRules.UrlBareAuthority,
                    "url must not contain credentials, a query, or a fragment"),
            };

        ServerAuthConfig auth;
        try
        {
            auth = MapAuth(document.Auth, entry.DisplayName);
        }
        catch (DesktopConfigException error)
        {
            return entry with
            {
                Violation = new ServerViolation(ServerRules.AuthVariant, error.Message),
            };
        }

        var canonicalUrl = CanonicalServerUrl(url);
        var admitted = entry with
        {
            Id = ServerIdentifier(canonicalUrl),
            Url = new Uri(canonicalUrl),
            Auth = auth,
            Admission = document.Enabled ? AdmissionStates.Usable : AdmissionStates.Disabled,
        };
        if (admitted.DefaultExecution && !admitted.Enabled)
            return admitted with
            {
                Admission = AdmissionStates.ConfigurationInvalid,
                Violation = new ServerViolation(
                    ServerRules.DefaultMustBeEnabled,
                    "a default execution entry must be enabled"),
            };
        return admitted;
    }

    private static readonly string[] EntryFields =
        ["id", "display_name", "enabled", "default_execution", "url", "auth"];

    private static string? UnknownEntryField(JsonElement raw)
    {
        foreach (var property in raw.EnumerateObject())
        {
            if (!EntryFields.Contains(property.Name, StringComparer.Ordinal))
                return property.Name;
        }
        return null;
    }

    private static string SalvageDisplayName(JsonElement raw)
    {
        if (raw.ValueKind == JsonValueKind.Object &&
            raw.TryGetProperty("display_name", out var name) &&
            name.ValueKind == JsonValueKind.String)
            return name.GetString() ?? "";
        return "";
    }

    // Uniqueness is exact code points after trimming: no case folding and no
    // Unicode normalisation, because those would require Go, TypeScript and C#
    // to agree on a fold — a new cross-language divergence in the very code
    // path this contract exists to make consistent.
    private static string DisplayNameKey(string name) => name.Trim();

    private static ServerViolation? CheckDisplayName(string name)
    {
        var trimmed = DisplayNameKey(name);
        if (trimmed.Length == 0)
            return new ServerViolation(
                ServerRules.DisplayNameRequired, "display_name must not be blank");
        foreach (var character in trimmed)
        {
            if (character <= 0x1f || character == 0x7f)
                return new ServerViolation(
                    ServerRules.DisplayNameCharset,
                    $"display_name contains control character U+{(int)character:X4}");
        }
        if (trimmed.StartsWith('-'))
            return new ServerViolation(
                ServerRules.DisplayNameCharset,
                "display_name must not begin with '-', which a command line reads as a flag");
        return null;
    }

    // No winner is chosen: a duplicate is a property of a pair, so blaming one
    // member requires choosing which, and every choice rule silently redirects
    // an operator's existing reference to a different machine.
    private static void MarkCollisions(
        List<ServerConfig> servers,
        Func<ServerConfig, string?> key,
        string rule,
        string detail)
    {
        var groups = new Dictionary<string, List<int>>(StringComparer.Ordinal);
        for (var index = 0; index < servers.Count; index++)
        {
            if (servers[index].Violation is not null)
                continue;
            var value = key(servers[index]);
            if (string.IsNullOrEmpty(value))
                continue;
            if (!groups.TryGetValue(value, out var members))
            {
                members = [];
                groups[value] = members;
            }
            members.Add(index);
        }
        foreach (var members in groups.Values)
        {
            if (members.Count < 2)
                continue;
            foreach (var index in members)
                servers[index] = servers[index] with
                {
                    Admission = AdmissionStates.ConfigurationInvalid,
                    Violation = new ServerViolation(rule, detail),
                };
        }
    }

    private static void MarkDefaultExecution(List<ServerConfig> servers)
    {
        var marked = new List<int>();
        for (var index = 0; index < servers.Count; index++)
        {
            if (servers[index].Violation is null && servers[index].DefaultExecution)
                marked.Add(index);
        }
        if (marked.Count > 1)
        {
            foreach (var index in marked)
                servers[index] = servers[index] with
                {
                    Admission = AdmissionStates.ConfigurationInvalid,
                    Violation = new ServerViolation(
                        ServerRules.AtMostOneDefault,
                        "more than one entry is marked as the default execution server"),
                };
            // Every marker is now unusable, so the legacy fallback below applies.
            marked.Clear();
        }
        if (marked.Count == 1)
            return;
        for (var index = 0; index < servers.Count; index++)
        {
            if (servers[index].IsUsable)
            {
                servers[index] = servers[index] with { DefaultExecution = true };
                return;
            }
        }
    }

    private static ServerAuthConfig MapAuth(AuthDocument? auth, string displayName)
    {
        if (auth is null)
            throw AuthError(displayName);
        return auth.Mode switch
        {
            "none" when auth.Token is null => ServerAuthConfig.None,
            "bearer" when !string.IsNullOrWhiteSpace(auth.Token) =>
                ServerAuthConfig.Bearer(auth.Token.Trim()),
            "none" => throw new DesktopConfigException(
                $"servers.json: server '{displayName}' auth mode none cannot contain a token"),
            "bearer" => throw new DesktopConfigException(
                $"servers.json: server '{displayName}' bearer auth needs a non-empty token"),
            _ => throw AuthError(displayName),
        };
    }

    private static DesktopConfigException AuthError(string displayName) => new(
        $"servers.json: server '{displayName}' auth must be {{\"mode\":\"none\"}} or " +
        "{\"mode\":\"bearer\",\"token\":\"...\"}");

    // The folding every client loader must agree on: scheme and host case, a
    // default port, and trailing path slashes. Credentials, a query and a
    // fragment are not folded away here — MapServer rejects an entry carrying
    // them before this is reached.
    public static string CanonicalServerUrl(Uri url)
    {
        var canonical = url.GetComponents(
            UriComponents.SchemeAndServer | UriComponents.Path,
            UriFormat.UriEscaped);
        return canonical.TrimEnd('/');
    }

    public static string ServerIdentifier(string canonicalUrl)
    {
        var digest = SHA256.HashData(Encoding.UTF8.GetBytes(canonicalUrl));
        return $"srv_{Convert.ToHexString(digest).ToLowerInvariant()}";
    }

    private static void ValidateAppearance(AppearanceDocument value)
    {
        if (value.Theme is not ("default" or "system" or "light" or "dark"))
            throw new DesktopConfigException(
                "appearance.json: theme must be default, system, light or dark");
        if (value.Density is not ("compact" or "comfortable"))
            throw new DesktopConfigException("appearance.json: density must be compact or comfortable");
        if (value.FontScale is < 0.8 or > 1.5)
            throw new DesktopConfigException("appearance.json: font_scale must be between 0.8 and 1.5");
    }

    private static void ValidateShell(ShellDocument value)
    {
        if (string.IsNullOrWhiteSpace(value.WorkspaceExecutable))
            throw new DesktopConfigException("shell.json: workspace_executable is required");
        if (value.HealthPollIntervalSeconds is < 1 or > 300)
            throw new DesktopConfigException(
                "shell.json: health_poll_interval_seconds must be between 1 and 300");
    }

    private static void ValidateWorkspace(WorkspaceDocument value)
    {
        if (value.IdleQuitSeconds is < 0 or > 86400)
            throw new DesktopConfigException(
                "workspace.json: idle_quit_seconds must be between 0 and 86400");
        if (value.DefaultWindow.Width is < 320 or > 16384 ||
            value.DefaultWindow.Height is < 240 or > 16384)
            throw new DesktopConfigException("workspace.json: default_window size is out of range");
    }

    private static void ValidateVersion(int version, string name, int expected = SchemaVersion)
    {
        if (version != expected)
            throw new DesktopConfigException(
                $"{name}: unsupported schema_version {version}; expected {expected}" +
                (name == "servers.json"
                    ? ". Use auth {\"mode\":\"none\"} or {\"mode\":\"bearer\",\"token\":\"...\"}; token_path is no longer supported."
                    : ""));
    }

    private static T Read<T>(string directory, string name)
    {
        var path = Path.Combine(directory, name);
        try
        {
            var json = File.ReadAllText(path);
            if (name == "servers.json")
            {
                using var document = JsonDocument.Parse(json);
                if (!document.RootElement.TryGetProperty("schema_version", out var version) ||
                    version.ValueKind != JsonValueKind.Number ||
                    version.GetInt32() != ServersSchemaVersion)
                {
                    var actual = version.ValueKind == JsonValueKind.Undefined
                        ? "missing"
                        : version.ToString();
                    throw new DesktopConfigException(
                        $"servers.json: unsupported schema_version {actual}; expected {ServersSchemaVersion}. " +
                        "Use auth {\"mode\":\"none\"} or {\"mode\":\"bearer\",\"token\":\"...\"}; " +
                        "token_path is no longer supported.");
                }
            }
            return JsonSerializer.Deserialize<T>(json, JsonOptions)
                   ?? throw new DesktopConfigException($"{name}: document is null");
        }
        catch (JsonException ex)
        {
            throw new DesktopConfigException($"{name}: invalid JSON: {ex.Message}", ex);
        }
        catch (IOException ex)
        {
            throw new DesktopConfigException($"{name}: cannot read file: {ex.Message}", ex);
        }
    }

    private static void CreateIfMissing(string path, object value)
    {
        if (File.Exists(path))
            return;
        var json = JsonSerializer.Serialize(value, JsonOptions) + Environment.NewLine;
        try
        {
            using var stream = new FileStream(path, FileMode.CreateNew, FileAccess.Write, FileShare.Read);
            using var writer = new StreamWriter(stream);
            writer.Write(json);
            writer.Flush();
            stream.Flush(flushToDisk: true);
            if (!OperatingSystem.IsWindows())
                File.SetUnixFileMode(path, UnixFileMode.UserRead | UnixFileMode.UserWrite);
        }
        catch (IOException) when (File.Exists(path))
        {
            // The other desktop process won first-start creation.
        }
    }

    private static IReadOnlyDictionary<string, object> DefaultDocuments()
    {
        return new Dictionary<string, object>
        {
            ["servers.json"] = new ServersDocument(
                ServersSchemaVersion,
                [
                    new ServerDocument(
                        "Local",
                        true,
                        true,
                        "http://127.0.0.1:8443",
                        new AuthDocument("none", null))
                ]),
            ["appearance.json"] = new AppearanceDocument(SchemaVersion, "default", "comfortable", 1.0),
            ["shell.json"] = new ShellDocument(SchemaVersion, "agent-grid-workspace", 5, null),
            ["workspace.json"] = new WorkspaceDocument(
                SchemaVersion,
                300,
                new WindowSizeDocument(1280, 800)),
        };
    }

    private sealed record ServersDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("servers")] IReadOnlyList<ServerDocument> Servers);

    // Reading keeps entries as raw elements so each is decoded on its own: an
    // unrecognised field or a value of the wrong type must cost one entry, and
    // a whole-document decode would make it cost the fleet.
    private sealed record ServersReadDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("servers")] JsonElement[]? Servers);

    private sealed record ServerDocument(
        [property: JsonPropertyName("display_name")] string DisplayName,
        [property: JsonPropertyName("enabled")] bool Enabled,
        [property: JsonPropertyName("default_execution")]
        [property: JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        bool? DefaultExecution,
        [property: JsonPropertyName("url")] string Url,
        [property: JsonPropertyName("auth")] AuthDocument Auth,
        // Read-only compatibility for schema v1 files that persisted a
        // client-selected routing identifier. The value is never mapped or written.
        [property: JsonPropertyName("id")]
        [property: JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        JsonElement? LegacyId = null);

    private sealed record AuthDocument(
        [property: JsonPropertyName("mode")] string Mode,
        [property: JsonPropertyName("token")]
        [property: JsonIgnore(Condition = JsonIgnoreCondition.WhenWritingNull)]
        string? Token);

    private sealed record AppearanceDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("theme")] string Theme,
        [property: JsonPropertyName("density")] string Density,
        [property: JsonPropertyName("font_scale")] double FontScale);

    private sealed record ShellDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("workspace_executable")] string WorkspaceExecutable,
        [property: JsonPropertyName("health_poll_interval_seconds")] int HealthPollIntervalSeconds,
        [property: JsonPropertyName("display_anchor")] DisplayAnchorDocument? DisplayAnchor = null);

    private sealed record DisplayAnchorDocument(
        [property: JsonPropertyName("x")] int X,
        [property: JsonPropertyName("y")] int Y);

    private sealed record WorkspaceDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("idle_quit_seconds")] int IdleQuitSeconds,
        [property: JsonPropertyName("default_window")] WindowSizeDocument DefaultWindow);

    private sealed record WindowSizeDocument(
        [property: JsonPropertyName("width")] int Width,
        [property: JsonPropertyName("height")] int Height);
}

public sealed class DesktopConfigException : Exception
{
    public DesktopConfigException(string message) : base(message) {}
    public DesktopConfigException(string message, Exception inner) : base(message, inner) {}
}
