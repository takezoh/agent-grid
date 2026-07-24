using System.Text.Json;
using AgentGrid.Shell.Core.Configuration;

namespace AgentGrid.Shell.Core.Tests.Configuration;

public sealed class DesktopConfigTests : IDisposable
{
    private readonly string _directory =
        Path.Combine(Path.GetTempPath(), $"agent-grid-config-{Guid.NewGuid():N}");

    [Fact]
    public void Missing_files_are_created_and_loaded()
    {
        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Equal(["local"], config.Servers.Select(s => s.Id));
        Assert.Equal("default", config.Appearance.Theme);
        Assert.Equal("agent-grid-workspace", config.Shell.WorkspaceExecutable);
        Assert.All(
            ["servers.json", "appearance.json", "shell.json", "workspace.json"],
            name => Assert.True(File.Exists(Path.Combine(_directory, name))));
    }

    [Fact]
    public void Config_directory_argument_overrides_default()
    {
        var resolved = DesktopConfigLoader.ResolveConfigDirectory(
            ["--unrelated", "x", "--config-dir", _directory]);

        Assert.Equal(Path.GetFullPath(_directory), resolved);
    }

    [Fact]
    public void Multiple_servers_are_loaded_with_stable_ids()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "servers.json");
        var document = new
        {
            schema_version = 1,
            servers = new object[]
            {
                Server("one", "http://127.0.0.1:8443"),
                Server("two", "https://example.test"),
            },
        };
        File.WriteAllText(path, JsonSerializer.Serialize(document));

        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Equal(["one", "two"], config.Servers.Select(s => s.Id));
        Assert.All(config.Servers, server => Assert.Equal("connect_only", server.Launch.Mode));
    }

    [Fact]
    public void Invalid_file_fails_instead_of_overwriting_user_config()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "appearance.json");
        File.WriteAllText(path, """{"schema_version":1,"theme":"blue"}""");

        var error = Assert.Throws<DesktopConfigException>(
            () => DesktopConfigLoader.LoadOrCreate(_directory));

        Assert.Contains("appearance.json", error.Message);
        Assert.Equal("""{"schema_version":1,"theme":"blue"}""", File.ReadAllText(path));
    }

    public void Dispose()
    {
        if (Directory.Exists(_directory))
            Directory.Delete(_directory, recursive: true);
    }

    private static object Server(string id, string url) => new
    {
        id,
        display_name = id,
        enabled = true,
        url,
        token_path = "token",
        launch = new { mode = "connect_only" },
    };
}
