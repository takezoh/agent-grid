using System.Text.RegularExpressions;

namespace AgentGrid.Shell.Core.DeepLinkRouter;

/// <summary>
/// Pure agent-grid:// URI → routing decision.
/// Sole protocol-handler logic owner; owns schema-gap for question/jump
/// (contract-deep-link-question-jump-kind-gap).
/// </summary>
public static partial class DeepLinkRouter
{
    // Typed kinds currently accepted by protocol/deep-links.schema.json.
    private static readonly Regex TypedSession =
        TypedSessionRegex();
    private static readonly Regex TypedApproval =
        TypedApprovalRegex();

    // Track A aliases (interim).
    private static readonly Regex AliasQuestion =
        AliasQuestionRegex();
    private static readonly Regex AliasSessionJump =
        AliasSessionJumpRegex();

    public static RoutingDecision Route(string uri)
    {
        if (string.IsNullOrWhiteSpace(uri))
            return RoutingDecision.Reject("empty uri");

        uri = uri.Trim();

        var m = TypedSession.Match(uri);
        if (m.Success)
        {
            return new RoutingDecision(
                RouteKind.OpenWorkspaceSession,
                m.Groups[1].Value,
                ItemKind: "session",
                Source: "typed-helper");
        }

        m = TypedApproval.Match(uri);
        if (m.Success)
        {
            return new RoutingDecision(
                RouteKind.PanelFocusItem,
                m.Groups[1].Value,
                ItemKind: "approval",
                Source: "typed-helper");
        }

        // Alias: agent-grid://question/<id> → panel focus (question pathway).
        m = AliasQuestion.Match(uri);
        if (m.Success)
        {
            return new RoutingDecision(
                RouteKind.PanelFocusItem,
                m.Groups[1].Value,
                ItemKind: "question",
                Source: "alias");
        }

        // Alias: agent-grid://session/<id>/jump → jump-back (router-local, not on wire).
        m = AliasSessionJump.Match(uri);
        if (m.Success)
        {
            return new RoutingDecision(
                RouteKind.JumpBack,
                m.Groups[1].Value,
                ItemKind: "session",
                Source: "alias");
        }

        // Do not invent new kind strings for wire emission; reject unknowns.
        return RoutingDecision.Reject($"unrecognized agent-grid uri: {uri}");
    }

    [GeneratedRegex(@"^agent-grid://session/([^/?#]+)$", RegexOptions.CultureInvariant)]
    private static partial Regex TypedSessionRegex();

    [GeneratedRegex(@"^agent-grid://approval/([^/?#]+)$", RegexOptions.CultureInvariant)]
    private static partial Regex TypedApprovalRegex();

    [GeneratedRegex(@"^agent-grid://question/([^/?#]+)$", RegexOptions.CultureInvariant)]
    private static partial Regex AliasQuestionRegex();

    [GeneratedRegex(@"^agent-grid://session/([^/?#]+)/jump$", RegexOptions.CultureInvariant)]
    private static partial Regex AliasSessionJumpRegex();
}
