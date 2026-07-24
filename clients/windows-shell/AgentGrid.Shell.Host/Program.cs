using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DeepLinkRouter;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Platform.JumpBack;

namespace AgentGrid.Shell.Host;

/// <summary>
/// Headless Windows host: wires composition root, runs health ticks, routes
/// deep links from argv. Replaceable by WinUI tray/panel host later
/// (shell-host-framework-choice invariance).
/// </summary>
public static class Program
{
    public static async Task<int> Main(string[] args)
    {
        var opts = ParseArgs(args);
        if (opts.ShowHelp)
        {
            PrintHelp();
            return 0;
        }

        if (opts.DeepLink is not null)
        {
            var decision = DeepLinkRouter.Route(opts.DeepLink);
            Console.WriteLine($"deep-link: {decision.Kind} id={decision.Id} item={decision.ItemKind} src={decision.Source}");
            if (decision.Kind == RouteKind.Rejected)
                return 2;
            // Fall through to start host and act on the link after connect.
        }

        var gatewayUri = opts.GatewayUri ?? new Uri($"http://127.0.0.1:{opts.Port}");
        var tokenPath = opts.TokenPath
            ?? Path.Combine(
                Environment.GetEnvironmentVariable("LOCALAPPDATA")
                ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),
                "agent-grid",
                "gateway-token");

        await using var root = ShellCompositionRoot.Build(new ShellHostOptions
        {
            GatewayBaseUri = gatewayUri,
            TokenPath = tokenPath,
            TokenPathInWsl = opts.TokenPathInWsl,
            WslDistro = opts.WslDistro,
            ServerPathInWsl = opts.ServerPath,
            GatewayPort = opts.Port,
            WorkspaceExePath = opts.WorkspaceExe,
            WorkspaceControlPath = opts.ControlPath,
            QuitApplication = () => Environment.Exit(0),
        });

        Console.CancelKeyPress += (_, e) =>
        {
            e.Cancel = true;
            root.Menu.OnQuit();
        };

        Console.WriteLine($"Agent Grid Shell Host — gateway {gatewayUri}");
        await root.StartAsync().ConfigureAwait(false);

        using var healthTimer = new PeriodicTimer(TimeSpan.FromSeconds(5));
        var cts = new CancellationTokenSource();
        Console.CancelKeyPress += (_, e) =>
        {
            e.Cancel = true;
            cts.Cancel();
        };

        if (opts.DeepLink is not null)
            await HandleDeepLinkAsync(root, opts.DeepLink, cts.Token).ConfigureAwait(false);

        while (await healthTimer.WaitForNextTickAsync(cts.Token).ConfigureAwait(false))
        {
            try
            {
                await root.Supervisor.HealthTickAsync(cts.Token).ConfigureAwait(false);
            }
            catch (Exception ex)
            {
                Console.Error.WriteLine($"health tick: {ex.Message}");
            }

            var tray = root.CurrentTrayAppearance();
            var glance = root.CurrentGlance();
            Console.WriteLine(
                $"[{DateTimeOffset.Now:HH:mm:ss}] tray={tray.Kind} conn={tray.IsConnected} " +
                $"pending={glance.PendingCount} status={glance.StatusLine}");
        }

        return 0;
    }

    private static async Task HandleDeepLinkAsync(
        ShellCompositionRoot root,
        string uri,
        CancellationToken ct)
    {
        var d = DeepLinkRouter.Route(uri);
        switch (d.Kind)
        {
            case RouteKind.OpenWorkspaceSession when d.Id is not null:
                var reply = await root.WorkspaceLauncher.OpenSessionAsync(d.Id, ct)
                    .ConfigureAwait(false);
                Console.WriteLine($"openSession {d.Id}: ok={reply.Ok} err={reply.Error}");
                break;
            case RouteKind.JumpBack when d.Id is not null:
                var jb = root.JumpBack.Jump(new JumpBackTarget(
                    CachedHwnd: null,
                    ProcessName: "WindowsTerminal",
                    TitleContains: null,
                    CwdHint: null));
                Console.WriteLine($"jump-back {d.Id}: {jb.Outcome} {jb.Detail}");
                break;
            case RouteKind.PanelFocusItem:
                Console.WriteLine($"panel-focus {d.ItemKind}/{d.Id} (headless: logged only)");
                break;
        }
    }

    private static HostArgs ParseArgs(string[] args)
    {
        var a = new HostArgs();
        for (var i = 0; i < args.Length; i++)
        {
            switch (args[i])
            {
                case "-h" or "--help":
                    a.ShowHelp = true;
                    break;
                case "--gateway" when i + 1 < args.Length:
                    a.GatewayUri = new Uri(args[++i]);
                    break;
                case "--token-path" when i + 1 < args.Length:
                    a.TokenPath = args[++i];
                    break;
                case "--token-path-wsl" when i + 1 < args.Length:
                    a.TokenPathInWsl = args[++i];
                    break;
                case "--distro" when i + 1 < args.Length:
                    a.WslDistro = args[++i];
                    break;
                case "--server" when i + 1 < args.Length:
                    a.ServerPath = args[++i];
                    break;
                case "--port" when i + 1 < args.Length:
                    a.Port = int.Parse(args[++i]);
                    break;
                case "--workspace-exe" when i + 1 < args.Length:
                    a.WorkspaceExe = args[++i];
                    break;
                case "--control-path" when i + 1 < args.Length:
                    a.ControlPath = args[++i];
                    break;
                case "--deep-link" when i + 1 < args.Length:
                    a.DeepLink = args[++i];
                    break;
                default:
                    if (args[i].StartsWith("agent-grid://", StringComparison.OrdinalIgnoreCase))
                        a.DeepLink = args[i];
                    break;
            }
        }

        // Defaults: Windows token path under LOCALAPPDATA when present.
        if (a.TokenPath is null)
        {
            var local = Environment.GetEnvironmentVariable("LOCALAPPDATA")
                        ?? Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData);
            a.TokenPath = Path.Combine(local, "agent-grid", "gateway-token");
        }

        a.GatewayUri ??= new Uri($"http://127.0.0.1:{a.Port}");
        return a;
    }

    private static void PrintHelp()
    {
        Console.WriteLine("""
            AgentGrid.Shell.Host — headless Phase 2 shell composition host

            Options:
              --gateway <url>         Gateway base (default http://127.0.0.1:8443)
              --token-path <path>     Windows-side token file (fresh-read each probe)
              --token-path-wsl <p>    Path inside WSL for daemon -token-file
              --distro <name>         WSL distro (default Ubuntu-22.04)
              --server <path>         server binary path inside WSL
              --port <n>              Gateway port (default 8443)
              --workspace-exe <path>  Workspace Electron executable
              --control-path <path>   Named pipe / socket for Workspace control
              --deep-link <uri>       agent-grid://… to route on start
              -h, --help              This help
            """);
    }

    private sealed class HostArgs
    {
        public bool ShowHelp;
        public Uri? GatewayUri;
        public string? TokenPath;
        public string TokenPathInWsl = "~/.agent-grid/gateway-token";
        public string WslDistro = "Ubuntu-22.04";
        public string ServerPath = "/workspace/agent-grid/server";
        public int Port = 8443;
        public string WorkspaceExe = "agent-grid-workspace";
        public string? ControlPath;
        public string? DeepLink;
    }
}
