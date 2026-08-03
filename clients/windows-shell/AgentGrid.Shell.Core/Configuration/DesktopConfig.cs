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

public sealed record ServerConfig(
    string Id,
    string DisplayName,
    bool Enabled,
    bool DefaultExecution,
    Uri Url,
    ServerAuthConfig Auth);

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

        var servers = Read<ServersDocument>(configDirectory, "servers.json");
        var appearance = Read<AppearanceDocument>(configDirectory, "appearance.json");
        var shell = Read<ShellDocument>(configDirectory, "shell.json");
        var workspace = Read<WorkspaceDocument>(configDirectory, "workspace.json");

        ValidateVersion(servers.SchemaVersion, "servers.json", ServersSchemaVersion);
        ValidateVersion(appearance.SchemaVersion, "appearance.json");
        ValidateVersion(shell.SchemaVersion, "shell.json");
        ValidateVersion(workspace.SchemaVersion, "workspace.json");

        var explicitDefaults = servers.Servers
            .Select((server, index) => (Server: server, Index: index))
            .Where(item => item.Server.DefaultExecution == true)
            .ToList();
        if (explicitDefaults.Count > 1)
            throw new DesktopConfigException(
                "servers.json: at most one server can be the default execution server");
        if (explicitDefaults.Any(item => !item.Server.Enabled))
            throw new DesktopConfigException(
                "servers.json: the default execution server must be enabled");
        var fallbackDefaultIndex = explicitDefaults.Count == 0
            ? servers.Servers.ToList().FindIndex(server => server.Enabled)
            : -1;
        var mappedServers = servers.Servers
            .Select((server, index) => MapServer(
                server,
                server.DefaultExecution == true || index == fallbackDefaultIndex))
            .ToList();
        if (mappedServers.Count == 0)
            throw new DesktopConfigException("servers.json: at least one server is required");
        if (mappedServers.All(s => !s.Enabled))
            throw new DesktopConfigException("servers.json: at least one server must be enabled");
        if (mappedServers.Select(s => s.Id).Distinct(StringComparer.Ordinal).Count() != mappedServers.Count)
            throw new DesktopConfigException("servers.json: server ids must be unique");

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

    private static ServerConfig MapServer(ServerDocument server, bool defaultExecution)
    {
        if (string.IsNullOrWhiteSpace(server.DisplayName))
            throw new DesktopConfigException("servers.json: every server needs display_name");
        if (!Uri.TryCreate(server.Url, UriKind.Absolute, out var url) ||
            url.Scheme is not ("http" or "https"))
            throw new DesktopConfigException(
                $"servers.json: server '{server.DisplayName}' has invalid url");
        var auth = MapAuth(server.Auth, server.DisplayName);
        var canonicalUrl = CanonicalServerUrl(url);
        return new ServerConfig(
            ServerIdentifier(canonicalUrl),
            server.DisplayName,
            server.Enabled,
            defaultExecution,
            new Uri(canonicalUrl),
            auth);
    }

    private static ServerAuthConfig MapAuth(AuthDocument auth, string displayName)
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

    public static string CanonicalServerUrl(Uri url)
    {
        var builder = new UriBuilder(url)
        {
            UserName = "",
            Password = "",
            Query = "",
            Fragment = "",
        };
        var canonical = builder.Uri.GetComponents(
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
