using AgentGrid.Shell.Core.SupervisionState;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Media;
using Windows.UI;

namespace AgentGrid.Shell.WinUI.Panel;

/// <summary>
/// Maps PanelPresentation's semantic accent tokens to concrete brushes.
/// Instances are created lazily on the UI thread (first Render call).
/// </summary>
internal static class PanelBrushes
{
    public static SolidColorBrush Success { get; } = FromHex(0x34, 0xD3, 0x99);
    public static SolidColorBrush Warning { get; } = FromHex(0xFB, 0xBF, 0x24);
    public static SolidColorBrush Danger { get; } = FromHex(0xF8, 0x71, 0x71);
    public static SolidColorBrush Info { get; } = FromHex(0x60, 0xA5, 0xFA);
    public static SolidColorBrush Muted { get; } = FromHex(0x9A, 0xA3, 0xB2);

    public static SolidColorBrush ForToken(string token) => token switch
    {
        "success" => Success,
        "warning" => Warning,
        "danger" => Danger,
        "info" => Info,
        _ => Muted,
    };

    private static SolidColorBrush FromHex(byte r, byte g, byte b) =>
        new(Color.FromArgb(0xFF, r, g, b));
}

/// <summary>Binding projection of a pending PanelGlanceItem card.</summary>
public sealed class PanelItemVm
{
    public required PanelGlanceItem Item { get; init; }
    public required string Headline { get; init; }
    public required string KindLabel { get; init; }
    public required string SessionShortId { get; init; }
    public required string Glyph { get; init; }
    public required Brush AccentBrush { get; init; }
    public required Visibility ApprovalActionsVisibility { get; init; }
    public required Visibility QuestionHintVisibility { get; init; }

    public static PanelItemVm From(PanelGlanceItem item) => new()
    {
        Item = item,
        Headline = item.Headline,
        KindLabel = PanelPresentation.KindLabel(item.Kind),
        SessionShortId = string.IsNullOrEmpty(item.ServerId)
            ? PanelPresentation.ShortSessionId(item.SessionId)
            : $"{item.ServerId} · {PanelPresentation.ShortSessionId(item.SessionId)}",
        Glyph = item.Kind switch
        {
            "approval" => "\uE7BA", // Warning
            "question" => "\uE9CE", // Unknown (circled ?)
            _ => "\uE946", // Info
        },
        AccentBrush = PanelBrushes.ForToken(PanelPresentation.KindAccent(item.Kind)),
        ApprovalActionsVisibility =
            item.Kind == "approval" ? Visibility.Visible : Visibility.Collapsed,
        QuestionHintVisibility =
            item.Kind == "question" ? Visibility.Visible : Visibility.Collapsed,
    };
}

/// <summary>Binding projection of a session chip in the horizontal strip.</summary>
public sealed class SessionChipVm
{
    public required SessionSummary Session { get; init; }
    public required string Label { get; init; }
    public required string PhaseLabel { get; init; }
    public required Brush PhaseBrush { get; init; }

    public static SessionChipVm From(SessionSummary session) => new()
    {
        Session = session,
        Label = string.IsNullOrEmpty(session.ServerId)
            ? PanelPresentation.SessionLabel(session)
            : $"{session.ServerId} · {PanelPresentation.SessionLabel(session)}",
        PhaseLabel = PanelPresentation.PhaseLabel(session.Phase),
        PhaseBrush = PanelBrushes.ForToken(PanelPresentation.PhaseAccent(session.Phase)),
    };
}
