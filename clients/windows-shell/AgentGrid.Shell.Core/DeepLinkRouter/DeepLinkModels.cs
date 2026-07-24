namespace AgentGrid.Shell.Core.DeepLinkRouter;

/// <summary>
/// Routing decision produced by the pure DeepLinkRouter.
/// </summary>
public enum RouteKind
{
    /// <summary>Focus a panel queue item (approval or question via alias).</summary>
    PanelFocusItem,

    /// <summary>Hand off to WorkspaceLauncher (open/focus session window).</summary>
    OpenWorkspaceSession,

    /// <summary>Jump-back to external target window for the session.</summary>
    JumpBack,

    /// <summary>URI rejected (malformed or unknown kind not covered by alias).</summary>
    Rejected,
}

public sealed record RoutingDecision(
    RouteKind Kind,
    string? ServerId,
    string? Id,
    string? ItemKind, // "approval" | "question" | "session" | null
    string Source,    // "typed-helper" | "alias" | "rejected"
    string? Reason = null)
{
    public static RoutingDecision Reject(string reason) =>
        new(RouteKind.Rejected, null, null, null, "rejected", reason);
}

/// <summary>
/// Interim client-side alias for kinds absent from protocol/deep-links.schema.json.
/// Documented as removable once Track B additive schema PR lands
/// (adr-20260724-deep-link-schema-additive-extension).
/// </summary>
public static class AliasTable
{
    // Track A interim: question → panel-focus pathway (same UI as approval).
    // server/<server-id>/session/<id>/jump → router-local hint.
    public const string QuestionPathPrefix = "question/";
    public const string JumpSuffix = "/jump";
}
