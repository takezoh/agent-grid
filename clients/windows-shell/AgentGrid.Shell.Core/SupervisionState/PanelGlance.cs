namespace AgentGrid.Shell.Core.SupervisionState;

/// <summary>
/// Glance-mode projection of SupervisionSnapshot for the always-visible bar
/// and tray flyout (S2 panel glance). Pure; UI binds to this only.
/// </summary>
public sealed record PanelGlanceItem(
    string ItemId,
    string SessionId,
    string Kind, // "approval" | "question" | "already-handled"
    string Headline,
    DateTimeOffset? ExpiresAt,
    string ServerId = "");

public sealed record PanelGlanceView(
    IReadOnlyList<SessionSummary> Sessions,
    IReadOnlyList<PanelGlanceItem> Pending,
    IReadOnlyList<PanelGlanceItem> Notices,
    int PendingCount,
    bool ConnectionFailed,
    string? ConnectionFailureReason,
    string StatusLine)
{
    public static PanelGlanceView From(SupervisionSnapshot snap)
    {
        var pending = new List<PanelGlanceItem>();
        foreach (var a in snap.Approvals)
        {
            pending.Add(new PanelGlanceItem(
                a.ApprovalId, a.SessionId, "approval", a.Summary, a.ExpiresAt, a.ServerId));
        }
        foreach (var q in snap.Questions)
        {
            pending.Add(new PanelGlanceItem(
                q.QuestionId, q.SessionId, "question", q.Prompt, q.ExpiresAt, q.ServerId));
        }

        var notices = snap.AlreadyHandled
            .Select(n => new PanelGlanceItem(
                n.ItemId, n.SessionId, "already-handled", n.Message, null, n.ServerId))
            .ToList();

        var status = snap.ConnectionFailed
            ? $"Disconnected: {snap.ConnectionFailureReason ?? "unknown"}"
            : pending.Count == 0
                ? $"{snap.Sessions.Count} session(s)"
                : $"{pending.Count} pending";

        return new PanelGlanceView(
            snap.Sessions,
            pending,
            notices,
            pending.Count,
            snap.ConnectionFailed,
            snap.ConnectionFailureReason,
            status);
    }
}
