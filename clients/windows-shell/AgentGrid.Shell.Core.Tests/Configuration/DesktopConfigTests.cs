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

        Assert.Equal(
            ["srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a"],
            config.Servers.Select(s => s.Id));
        Assert.True(Assert.Single(config.Servers).DefaultExecution);
        Assert.Equal("default", config.Appearance.Theme);
        Assert.Equal("agent-grid-workspace", config.Shell.WorkspaceExecutable);
        Assert.All(
            ["servers.json", "appearance.json", "shell.json", "workspace.json"],
            name => Assert.True(File.Exists(Path.Combine(_directory, name))));
        using var servers = JsonDocument.Parse(
            File.ReadAllText(Path.Combine(_directory, "servers.json")));
        Assert.False(servers.RootElement
            .GetProperty("servers")[0]
            .TryGetProperty("launch", out _));
        Assert.False(servers.RootElement
            .GetProperty("servers")[0]
            .TryGetProperty("id", out _));
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
            schema_version = 2,
            servers = new object[]
            {
                Server("one", "http://127.0.0.1:8443"),
                Server("two", "https://example.test"),
            },
        };
        File.WriteAllText(path, JsonSerializer.Serialize(document));

        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Equal(
            [
                "srv_ca5ed80c77e645b7c7ec83189f36bfa835128debf18096e46c6300fdc17e8c3a",
                "srv_f1e2c7da767e6e0fc3eac0819fdd3a622967f18748c6f0a875c3a5b0a165031e",
            ],
            config.Servers.Select(s => s.Id));
        Assert.Equal([true, false], config.Servers.Select(s => s.DefaultExecution));
    }

    [Fact]
    public void Explicit_default_execution_server_is_loaded()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "servers.json");
        File.WriteAllText(path, JsonSerializer.Serialize(new
        {
            schema_version = 2,
            servers = new object[]
            {
                Server("one", "http://127.0.0.1:8443"),
                new
                {
                    display_name = "two",
                    enabled = true,
                    default_execution = true,
                    url = "https://example.test",
                    auth = new { mode = "bearer", token = "token" },
                },
            },
        }));

        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Equal([false, true], config.Servers.Select(s => s.DefaultExecution));
    }

    [Fact]
    public void Invalid_default_execution_server_is_rejected()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "servers.json");
        File.WriteAllText(path, JsonSerializer.Serialize(new
        {
            schema_version = 2,
            servers = new object[]
            {
                new
                {
                    display_name = "one",
                    enabled = true,
                    default_execution = true,
                    url = "http://127.0.0.1:8443",
                    auth = new { mode = "bearer", token = "token" },
                },
                new
                {
                    display_name = "two",
                    enabled = true,
                    default_execution = true,
                    url = "https://example.test",
                    auth = new { mode = "bearer", token = "token" },
                },
            },
        }));

        var error = Assert.Throws<DesktopConfigException>(
            () => DesktopConfigLoader.LoadOrCreate(_directory));

        Assert.Contains("at most one", error.Message);
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

    [Fact]
    public void Legacy_schema_and_token_path_fail_without_rewrite()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "servers.json");
        File.WriteAllText(path, JsonSerializer.Serialize(new
        {
            schema_version = 1,
            servers = new[]
            {
                new
                {
                    id = "legacy",
                    display_name = "Legacy",
                    enabled = true,
                    url = "http://127.0.0.1:8443",
                    token_path = "token",
                    launch = new
                    {
                        mode = "managed_wsl",
                        wsl_distro = "Ubuntu",
                        server_path_in_wsl = "/tmp/server",
                        token_path_in_wsl = "/tmp/token",
                    },
                },
            },
        }));

        var before = File.ReadAllText(path);
        var error = Assert.Throws<DesktopConfigException>(
            () => DesktopConfigLoader.LoadOrCreate(_directory));

        Assert.Contains("schema_version 1", error.Message);
        Assert.Contains("token_path is no longer supported", error.Message);
        Assert.Equal(before, File.ReadAllText(path));
    }

    [Fact]
    public void Identifier_is_derived_from_the_cross_client_canonical_url_contract()
    {
        var canonical = DesktopConfigLoader.CanonicalServerUrl(
            new Uri("HTTP://Example.TEST:80/api///?token=ignored#fragment"));

        Assert.Equal("http://example.test/api", canonical);
        Assert.Equal(
            "srv_36edcc69555ffb4873eb2d2b461c03839096e08cbb4dd60ba5ecc53f8175a5d0",
            DesktopConfigLoader.ServerIdentifier(canonical));
    }

    [Theory]
    [InlineData("servers-valid-none.json", ServerAuthMode.None)]
    [InlineData("servers-valid-bearer.json", ServerAuthMode.Bearer)]
    public void Shared_valid_auth_fixtures_are_accepted(string fixture, ServerAuthMode expected)
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        File.Copy(
            Path.Combine(AppContext.BaseDirectory, "Fixtures", fixture),
            Path.Combine(_directory, "servers.json"),
            overwrite: true);

        var server = Assert.Single(DesktopConfigLoader.LoadOrCreate(_directory).Servers);
        Assert.Equal(expected, server.Auth.Mode);
    }

    [Theory]
    [InlineData("servers-invalid-v1-token-path.json")]
    [InlineData("servers-invalid-empty-bearer.json")]
    public void Shared_invalid_auth_fixtures_are_rejected(string fixture)
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        File.Copy(
            Path.Combine(AppContext.BaseDirectory, "Fixtures", fixture),
            Path.Combine(_directory, "servers.json"),
            overwrite: true);

        Assert.Throws<DesktopConfigException>(
            () => DesktopConfigLoader.LoadOrCreate(_directory));
    }

    public void Dispose()
    {
        if (Directory.Exists(_directory))
            Directory.Delete(_directory, recursive: true);
    }

    private static object Server(string displayName, string url) => new
    {
        display_name = displayName,
        enabled = true,
        url,
        auth = new { mode = "bearer", token = "token" },
    };
}
