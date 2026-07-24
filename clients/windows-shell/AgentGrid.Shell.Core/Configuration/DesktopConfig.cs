using System.Text.Json;
using System.Text.Json.Serialization;
using System.Text.RegularExpressions;

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
    Uri Url,
    string TokenPath,
    ServerLaunchConfig Launch);

public sealed record ServerLaunchConfig(
    string Mode,
    string? WslDistro,
    string? ServerPathInWsl,
    string? TokenPathInWsl);

public sealed record AppearanceConfig(string Theme, string Density, double FontScale);

public sealed record ShellAppConfig(string WorkspaceExecutable, int HealthPollIntervalSeconds);

public sealed record WorkspaceAppConfig(
    int IdleQuitSeconds,
    int DefaultWindowWidth,
    int DefaultWindowHeight);

/// <summary>
/// Loads the four desktop configuration files shared by Shell and Workspace.
/// Missing files are created from the same schema-versioned defaults. Existing
/// files are never overwritten.
/// </summary>
public static partial class DesktopConfigLoader
{
    public const int SchemaVersion = 1;
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

        ValidateVersion(servers.SchemaVersion, "servers.json");
        ValidateVersion(appearance.SchemaVersion, "appearance.json");
        ValidateVersion(shell.SchemaVersion, "shell.json");
        ValidateVersion(workspace.SchemaVersion, "workspace.json");

        var mappedServers = servers.Servers.Select(MapServer).ToList();
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
            new ShellAppConfig(shell.WorkspaceExecutable, shell.HealthPollIntervalSeconds),
            new WorkspaceAppConfig(
                workspace.IdleQuitSeconds,
                workspace.DefaultWindow.Width,
                workspace.DefaultWindow.Height),
            configDirectory);
    }

    private static ServerConfig MapServer(ServerDocument server)
    {
        if (!ServerIdRegex().IsMatch(server.Id))
            throw new DesktopConfigException(
                $"servers.json: invalid server id '{server.Id}' (use letters, digits, '.', '_' or '-')");
        if (string.IsNullOrWhiteSpace(server.DisplayName))
            throw new DesktopConfigException($"servers.json: server '{server.Id}' needs display_name");
        if (!Uri.TryCreate(server.Url, UriKind.Absolute, out var url) ||
            url.Scheme is not ("http" or "https"))
            throw new DesktopConfigException($"servers.json: server '{server.Id}' has invalid url");
        if (string.IsNullOrWhiteSpace(server.TokenPath))
            throw new DesktopConfigException($"servers.json: server '{server.Id}' needs token_path");
        if (server.Launch.Mode is not ("managed_wsl" or "connect_only"))
            throw new DesktopConfigException(
                $"servers.json: server '{server.Id}' launch.mode must be managed_wsl or connect_only");
        if (server.Launch.Mode == "managed_wsl" &&
            (string.IsNullOrWhiteSpace(server.Launch.WslDistro) ||
             string.IsNullOrWhiteSpace(server.Launch.ServerPathInWsl) ||
             string.IsNullOrWhiteSpace(server.Launch.TokenPathInWsl)))
            throw new DesktopConfigException(
                $"servers.json: managed_wsl server '{server.Id}' needs all WSL launch fields");

        return new ServerConfig(
            server.Id,
            server.DisplayName,
            server.Enabled,
            url,
            Environment.ExpandEnvironmentVariables(server.TokenPath),
            new ServerLaunchConfig(
                server.Launch.Mode,
                server.Launch.WslDistro,
                server.Launch.ServerPathInWsl,
                server.Launch.TokenPathInWsl));
    }

    private static void ValidateAppearance(AppearanceDocument value)
    {
        if (value.Theme is not ("system" or "light" or "dark"))
            throw new DesktopConfigException("appearance.json: theme must be system, light or dark");
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

    private static void ValidateVersion(int version, string name)
    {
        if (version != SchemaVersion)
            throw new DesktopConfigException(
                $"{name}: unsupported schema_version {version}; expected {SchemaVersion}");
    }

    private static T Read<T>(string directory, string name)
    {
        var path = Path.Combine(directory, name);
        try
        {
            return JsonSerializer.Deserialize<T>(File.ReadAllText(path), JsonOptions)
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
        }
        catch (IOException) when (File.Exists(path))
        {
            // The other desktop process won first-start creation.
        }
    }

    private static IReadOnlyDictionary<string, object> DefaultDocuments()
    {
        var localAppData = Environment.GetEnvironmentVariable("LOCALAPPDATA");
        if (string.IsNullOrWhiteSpace(localAppData))
            localAppData = Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
        return new Dictionary<string, object>
        {
            ["servers.json"] = new ServersDocument(
                SchemaVersion,
                [
                    new ServerDocument(
                        "local",
                        "Local",
                        true,
                        "http://127.0.0.1:8443",
                        Path.Combine(localAppData, "agent-grid", "gateway-token"),
                        new LaunchDocument(
                            "managed_wsl",
                            "Ubuntu-22.04",
                            "~/agent-grid/server",
                            "~/.agent-grid/gateway-token"))
                ]),
            ["appearance.json"] = new AppearanceDocument(SchemaVersion, "system", "comfortable", 1.0),
            ["shell.json"] = new ShellDocument(SchemaVersion, "agent-grid-workspace", 5),
            ["workspace.json"] = new WorkspaceDocument(
                SchemaVersion,
                300,
                new WindowSizeDocument(1280, 800)),
        };
    }

    [GeneratedRegex(@"^[A-Za-z0-9][A-Za-z0-9._-]{0,63}$", RegexOptions.CultureInvariant)]
    private static partial Regex ServerIdRegex();

    private sealed record ServersDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("servers")] IReadOnlyList<ServerDocument> Servers);

    private sealed record ServerDocument(
        [property: JsonPropertyName("id")] string Id,
        [property: JsonPropertyName("display_name")] string DisplayName,
        [property: JsonPropertyName("enabled")] bool Enabled,
        [property: JsonPropertyName("url")] string Url,
        [property: JsonPropertyName("token_path")] string TokenPath,
        [property: JsonPropertyName("launch")] LaunchDocument Launch);

    private sealed record LaunchDocument(
        [property: JsonPropertyName("mode")] string Mode,
        [property: JsonPropertyName("wsl_distro")] string? WslDistro,
        [property: JsonPropertyName("server_path_in_wsl")] string? ServerPathInWsl,
        [property: JsonPropertyName("token_path_in_wsl")] string? TokenPathInWsl);

    private sealed record AppearanceDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("theme")] string Theme,
        [property: JsonPropertyName("density")] string Density,
        [property: JsonPropertyName("font_scale")] double FontScale);

    private sealed record ShellDocument(
        [property: JsonPropertyName("schema_version")] int SchemaVersion,
        [property: JsonPropertyName("workspace_executable")] string WorkspaceExecutable,
        [property: JsonPropertyName("health_poll_interval_seconds")] int HealthPollIntervalSeconds);

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
