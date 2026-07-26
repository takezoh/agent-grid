namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>
/// UI-agnostic display mapping for the glance panel. Front-ends (WinUI today)
/// translate the semantic accent tokens ("success" | "warning" | "danger" |
/// "info" | "muted") into concrete brushes/glyphs so every client renders the
/// same state the same way. Pure; keep testable from AgentGrid.Shell.Core.Tests.
/// </summary>
public static class PanelPresentation
{
    /// <summary>Human label for a PanelGlanceItem.Kind.</summary>
    public static string KindLabel(string kind) => kind switch
    {
        "approval" => "Permission request",
        "question" => "Question",
        "already-handled" => "Handled elsewhere",
        _ => kind,
    };

    /// <summary>Semantic accent token for a PanelGlanceItem.Kind.</summary>
    public static string KindAccent(string kind) => kind switch
    {
        "approval" => "warning",
        "question" => "info",
        _ => "muted",
    };

    /// <summary>Compact session identifier for chips and card metadata.</summary>
    public static string ShortSessionId(string sessionId) =>
        sessionId.Length <= 8 ? sessionId : sessionId[..8];

    public static string PhaseLabel(SessionPhase phase) => phase switch
    {
        SessionPhase.Running => "Running",
        SessionPhase.Waiting => "Waiting",
        SessionPhase.Failed => "Failed",
        SessionPhase.Done => "Done",
        _ => phase.ToString(),
    };

    /// <summary>Semantic accent token for a session phase dot.</summary>
    public static string PhaseAccent(SessionPhase phase) => phase switch
    {
        SessionPhase.Running => "success",
        SessionPhase.Waiting => "warning",
        SessionPhase.Failed => "danger",
        _ => "muted",
    };

    /// <summary>Workspace StatusIcon visual kind for a desktop session phase.</summary>
    public static string PhaseVisualKind(SessionPhase phase) => phase switch
    {
        SessionPhase.Running => "running",
        SessionPhase.Waiting => "waiting",
        SessionPhase.Failed => "stopped",
        SessionPhase.Done => "idle",
        _ => "unknown",
    };

    public static string SessionVisualKind(SessionSummary session) =>
        session.Phase == SessionPhase.Failed ? "failed" :
        session.HumanInput ? "action" :
        session.Lifecycle == "stopped" ? "stopped" :
        PhaseVisualKind(session.Phase);

    public static string SessionStatusLabel(SessionSummary session) =>
        SessionVisualKind(session) switch
        {
            "running" => "Working",
            "action" => "Action required",
            "idle" => "Idle",
            "stopped" => "Stopped",
            "failed" => "Failed",
            _ => "Unknown",
        };

    public static string SessionStatusAccent(SessionSummary session) =>
        SessionVisualKind(session) switch
        {
            "running" => "success",
            "action" => "warning",
            "failed" => "danger",
            _ => "muted",
        };

    /// <summary>Clients render the server-owned title without local fallback.</summary>
    public static string SessionLabel(SessionSummary session) => session.Title ?? "";

    /// <summary>
    /// One-line aggregate for the compact island bar. Priority mirrors what the
    /// user must act on first: offline &gt; pending &gt; failed &gt; working &gt; done &gt; idle.
    /// </summary>
    public static CompactSummary Compact(PanelGlanceView view)
    {
        if (view.ConnectionFailed)
            return new CompactSummary("offline", "danger");
        if (view.PendingCount > 0)
            return new CompactSummary($"{view.PendingCount} waiting", "warning");

        var failed = view.Sessions.Count(s => s.Phase == SessionPhase.Failed);
        if (failed > 0)
            return new CompactSummary($"{failed} failed", "danger");

        var running = view.Sessions.Count(s => s.Phase == SessionPhase.Running);
        if (running > 0)
            return new CompactSummary($"{running} working", "success");

        if (view.Sessions.Count > 0 && view.Sessions.All(s => s.Phase == SessionPhase.Done))
            return new CompactSummary("done", "success");
        if (view.Sessions.Count > 0)
            return new CompactSummary($"{view.Sessions.Count} session(s)", "muted");
        return new CompactSummary("idle", "muted");
    }

    /// <summary>
    /// Which expanded-island sections exist for this snapshot. Vibe Island
    /// contract: content IS the UI — a section with nothing to show is not
    /// rendered at all (no headers, no empty-state filler, no "online" badge),
    /// and the content-hugging window shrinks around what remains.
    /// </summary>
    public static PanelSections Sections(PanelGlanceView view) => new(
        ShowSessions: view.Sessions.Count > 0,
        ShowNotice: view.Notices.Count > 0,
        ShowPending: view.PendingCount > 0,
        ShowReply: view.Pending.Any(p => p.Kind == "question"),
        ShowOffline: view.ConnectionFailed);
}

/// <summary>Compact island bar line: text + semantic accent token.</summary>
public sealed record CompactSummary(string Text, string AccentToken);

/// <summary>Expanded-island section visibility (content-defined, no filler).</summary>
public sealed record PanelSections(
    bool ShowSessions,
    bool ShowNotice,
    bool ShowPending,
    bool ShowReply,
    bool ShowOffline);
