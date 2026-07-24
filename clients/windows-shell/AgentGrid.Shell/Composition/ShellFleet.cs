using AgentGrid.Shell.Core.Configuration;
using AgentGrid.Shell.Core.DaemonSupervisor;
using AgentGrid.Shell.Core.GatewayClient;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.Core.SessionIdentity;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Platform.Engage;
using AgentGrid.Shell.Platform.Interop;
using AgentGrid.Shell.Platform.JumpBack;
using AgentGrid.Shell.Platform.Toast;

namespace AgentGrid.Shell.Composition;

/// <summary>
/// Owns one independent connection/supervisor per enabled configured server.
/// ServerId remains a desktop routing key; each child gateway receives only
/// SessionId on its already-selected connection.
/// </summary>
public sealed class ShellFleet : IAsyncDisposable
{
    private readonly IReadOnlyDictionary<string, ShellCompositionRoot> _roots;
    private readonly Action _quit;

    private ShellFleet(
        DesktopConfig config,
        IReadOnlyDictionary<string, ShellCompositionRoot> roots,
        Action quit)
    {
        Config = config;
        _roots = roots;
        _quit = quit;
        var first = roots.Values.First();
        Win32 = first.Win32;
        JumpBack = first.JumpBack;
        Engage = first.Engage;
        ToastDecisions = first.ToastDecisions;
        foreach (var root in roots.Values)
        {
            root.Supervision.SnapshotChanged += _ => SnapshotChanged?.Invoke(CurrentSnapshot());
            root.Supervisor.SnapshotChanged += _ => HealthChanged?.Invoke(CurrentTrayAppearance());
        }
    }

    public DesktopConfig Config { get; }
    public IWin32InteropService Win32 { get; }
    public JumpBackService JumpBack { get; }
    public EngageFocusService Engage { get; }
    public ToastDecisionService ToastDecisions { get; }
    public event Action<SupervisionSnapshot>? SnapshotChanged;
    public event Action<TrayAppearance>? HealthChanged;

    public static ShellFleet Build(DesktopConfig config, Action quit)
    {
        var roots = new Dictionary<string, ShellCompositionRoot>(StringComparer.Ordinal);
        foreach (var server in config.Servers.Where(server => server.Enabled))
        {
            IDaemonRunner? runner = server.Launch.Mode == "connect_only"
                ? new ConnectOnlyDaemonRunner()
                : null;
            roots.Add(server.Id, ShellCompositionRoot.Build(new ShellHostOptions
            {
                GatewayBaseUri = server.Url,
                TokenPath = server.TokenPath,
                TokenPathInWsl = server.Launch.TokenPathInWsl ?? "~/.agent-grid/gateway-token",
                WslDistro = server.Launch.WslDistro ?? "Ubuntu-22.04",
                ServerPathInWsl = server.Launch.ServerPathInWsl ?? "~/agent-grid/server",
                GatewayPort = server.Url.Port,
                WorkspaceExePath = config.Shell.WorkspaceExecutable,
                WorkspaceArguments = [DesktopConfigLoader.ConfigDirectoryArgument, config.ConfigDirectory],
                DaemonRunner = runner,
                QuitApplication = quit,
            }));
        }
        return new ShellFleet(config, roots, quit);
    }

    public async Task StartAsync(CancellationToken ct = default)
    {
        await Task.WhenAll(_roots.Values.Select(root => root.StartAsync(ct))).ConfigureAwait(false);
    }

    public async Task HealthTickAsync(CancellationToken ct = default)
    {
        await Task.WhenAll(_roots.Values.Select(root => root.Supervisor.HealthTickAsync(ct)))
            .ConfigureAwait(false);
    }

    public SupervisionSnapshot CurrentSnapshot()
    {
        return AggregateSnapshots(
            _roots.Select(pair =>
                new KeyValuePair<string, SupervisionSnapshot>(
                    pair.Key,
                    pair.Value.Supervision.Snapshot)));
    }

    public static SupervisionSnapshot AggregateSnapshots(
        IEnumerable<KeyValuePair<string, SupervisionSnapshot>> source)
    {
        var snapshots = source.Select(pair => Tag(pair.Key, pair.Value)).ToList();
        var failures = snapshots
            .Where(snapshot => snapshot.ConnectionFailed)
            .Select(snapshot => snapshot.ConnectionFailureReason)
            .Where(reason => !string.IsNullOrWhiteSpace(reason))
            .ToList();
        return new SupervisionSnapshot(
            snapshots.SelectMany(snapshot => snapshot.Sessions).ToList(),
            snapshots.SelectMany(snapshot => snapshot.Approvals).ToList(),
            snapshots.SelectMany(snapshot => snapshot.Questions).ToList(),
            snapshots.SelectMany(snapshot => snapshot.AlreadyHandled).ToList(),
            ConnectionFailed: failures.Count > 0,
            ConnectionFailureReason: failures.Count == 0 ? null : string.Join("; ", failures));
    }

    public PanelGlanceView CurrentGlance() => PanelGlanceView.From(CurrentSnapshot());

    public TrayAppearance CurrentTrayAppearance()
    {
        var values = _roots.Values.Select(root => root.CurrentTrayAppearance()).ToList();
        var connected = values.Count(value => value.IsConnected);
        var total = values.Count;
        if (connected == total)
            return new TrayAppearance(TrayIconKind.Connected, $"Agent Grid — {connected}/{total} connected", true);
        if (connected > 0)
            return new TrayAppearance(TrayIconKind.Degraded, $"Agent Grid — {connected}/{total} connected", false);
        if (values.Any(value => value.Kind == TrayIconKind.Connecting))
            return new TrayAppearance(TrayIconKind.Connecting, "Agent Grid — connecting…", false);
        return new TrayAppearance(TrayIconKind.Degraded, "Agent Grid — all servers unavailable", false);
    }

    public Task OpenSessionAsync(ServerSessionId session, CancellationToken ct = default) =>
        Root(session.ServerId).WorkspaceLauncher.OpenSessionAsync(session, ct);

    public Task SubmitApprovalAsync(
        string serverId,
        string approvalId,
        string sessionId,
        string decision,
        string summary,
        DateTimeOffset? expiresAt = null,
        CancellationToken ct = default) =>
        Root(serverId).Supervision.SubmitApprovalAsync(
            approvalId, sessionId, decision, summary, expiresAt, ct);

    public Task SubmitQuestionAsync(
        string serverId,
        string questionId,
        string sessionId,
        string answer,
        string prompt,
        CancellationToken ct = default) =>
        Root(serverId).Supervision.SubmitQuestionAsync(
            questionId, sessionId, answer, prompt, ct);

    public Task RestartAllAsync(CancellationToken ct = default) =>
        Task.WhenAll(_roots.Values.Select(root => root.Supervisor.RestartAsync(ct)));

    public Task StopAllAsync(CancellationToken ct = default) =>
        Task.WhenAll(_roots.Values.Select(root => root.Supervisor.StopDaemonAsync(ct)));

    public void Quit() => _quit();

    public async ValueTask DisposeAsync()
    {
        foreach (var root in _roots.Values)
            await root.DisposeAsync().ConfigureAwait(false);
    }

    private ShellCompositionRoot Root(string serverId) =>
        _roots.TryGetValue(serverId, out var root)
            ? root
            : throw new KeyNotFoundException($"unknown or disabled server '{serverId}'");

    private static SupervisionSnapshot Tag(string serverId, SupervisionSnapshot snapshot) =>
        snapshot with
        {
            Sessions = snapshot.Sessions.Select(value => value with { ServerId = serverId }).ToList(),
            Approvals = snapshot.Approvals.Select(value => value with { ServerId = serverId }).ToList(),
            Questions = snapshot.Questions.Select(value => value with { ServerId = serverId }).ToList(),
            AlreadyHandled = snapshot.AlreadyHandled
                .Select(value => value with { ServerId = serverId })
                .ToList(),
            ConnectionFailureReason = snapshot.ConnectionFailed
                ? $"{serverId}: {snapshot.ConnectionFailureReason ?? "unknown"}"
                : null,
        };
}
