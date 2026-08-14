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
        Assert.Null(config.Shell.DisplayAnchor);
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
    public void Workspace_selected_display_anchor_is_loaded_for_shell_placement()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        var path = Path.Combine(_directory, "shell.json");
        File.WriteAllText(path, JsonSerializer.Serialize(new
        {
            schema_version = 1,
            workspace_executable = "agent-grid-workspace",
            health_poll_interval_seconds = 5,
            display_anchor = new { x = 3200, y = 720 },
        }));

        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Equal(new DisplayAnchor(3200, 720), config.Shell.DisplayAnchor);
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
    public void Ambiguous_default_execution_marker_invalidates_only_the_marked_group()
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

        // An ambiguous marker is entry-scoped under per-entry admission
        // (adr-20260814-server-display-name-naming-contract): it invalidates the
        // marked group and leaves the host running. The assertion is restated
        // against the state those entries must reach, not weakened.
        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.All(config.Servers, server =>
        {
            Assert.Equal(AdmissionStates.ConfigurationInvalid, server.Admission);
            Assert.Equal(ServerRules.AtMostOneDefault, server.Violation!.Rule);
        });
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

    // A query and a fragment are no longer folded away on the way to an
    // identifier — validation rejects an entry carrying them before this is
    // reached (adr-20260814-shared-go-client-config-module). What remains is
    // the folding every loader must agree on: scheme and host case, a default
    // port, and trailing path slashes.
    [Fact]
    public void Identifier_is_derived_from_the_cross_client_canonical_url_contract()
    {
        var canonical = DesktopConfigLoader.CanonicalServerUrl(
            new Uri("HTTP://Example.TEST:80/api///"));

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

    // The two shared invalid fixtures now separate along the admission
    // partition: a v1 document names no entry and stops the host, while an
    // empty bearer token names one and costs only that entry. The corpus in
    // ConfigCorpusTests covers the full partition.
    [Fact]
    public void Shared_v1_token_path_fixture_stops_the_host()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        File.Copy(
            Path.Combine(AppContext.BaseDirectory, "Fixtures", "servers-invalid-v1-token-path.json"),
            Path.Combine(_directory, "servers.json"),
            overwrite: true);

        Assert.Throws<DesktopConfigException>(
            () => DesktopConfigLoader.LoadOrCreate(_directory));
    }

    [Fact]
    public void Shared_empty_bearer_fixture_is_admitted_as_configuration_invalid()
    {
        DesktopConfigLoader.LoadOrCreate(_directory);
        File.Copy(
            Path.Combine(AppContext.BaseDirectory, "Fixtures", "servers-invalid-empty-bearer.json"),
            Path.Combine(_directory, "servers.json"),
            overwrite: true);

        var config = DesktopConfigLoader.LoadOrCreate(_directory);

        Assert.Contains(
            config.Servers,
            server => server.Admission == AdmissionStates.ConfigurationInvalid);
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
