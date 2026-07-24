using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.Configuration;
using AgentGrid.Shell.Core.DeepLinkRouter;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.SessionIdentity;
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

        var configDirectory = DesktopConfigLoader.ResolveConfigDirectory(args);
        var config = DesktopConfigLoader.LoadOrCreate(configDirectory);
        await using var root = ShellFleet.Build(config, () => Environment.Exit(0));

        Console.WriteLine(
            $"Agent Grid Shell Host — {config.Servers.Count(server => server.Enabled)} server(s)");
        await root.StartAsync().ConfigureAwait(false);

        using var healthTimer = new PeriodicTimer(
            TimeSpan.FromSeconds(config.Shell.HealthPollIntervalSeconds));
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
                await root.HealthTickAsync(cts.Token).ConfigureAwait(false);
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
        ShellFleet root,
        string uri,
        CancellationToken ct)
    {
        var d = DeepLinkRouter.Route(uri);
        switch (d.Kind)
        {
            case RouteKind.OpenWorkspaceSession when d.ServerId is not null && d.Id is not null:
                await root.OpenSessionAsync(new ServerSessionId(d.ServerId, d.Id), ct)
                    .ConfigureAwait(false);
                Console.WriteLine($"openSession {d.ServerId}/{d.Id}");
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
                case "--config-dir" when i + 1 < args.Length:
                    i++;
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

        return a;
    }

    private static void PrintHelp()
    {
        Console.WriteLine("""
            AgentGrid.Shell.Host — headless Phase 2 shell composition host

            Options:
              --config-dir <path>     Configuration directory override
              --deep-link <uri>       agent-grid://… to route on start
              -h, --help              This help
            """);
    }

    private sealed class HostArgs
    {
        public bool ShowHelp;
        public string? DeepLink;
    }
}
