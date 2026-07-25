using FlaUI.Core.AutomationElements;
using FlaUI.Core.Tools;

namespace AgentGrid.Shell.WinUI.UiTests.E2E;

/// <summary>
/// T3 opt-in UI facts against the real WinUI panel via UIA (FlaUI).
/// Harness: clients/windows-shell/scripts/e2e.sh (WinUI stage builds the exe,
/// launch-smoke verifies startup, then this suite drives the live window).
///
/// Vibe Island contract: sections exist only when they have content, so most
/// AutomationIds are state-dependent (SessionList/PendingList/EngageBox/
/// ConnectionText leave the UIA tree with their sections). Only the expanded
/// header chrome is unconditional.
/// </summary>
public sealed class PanelWindowUiE2ETests : IClassFixture<PanelUiSession>
{
    private readonly PanelUiSession _ui;

    public PanelWindowUiE2ETests(PanelUiSession ui) => _ui = ui;

    [WinUiUiFact]
    public void Panel_exposes_expanded_chrome_via_automation_ids()
    {
        // The unconditional AutomationId contract of the expanded island —
        // the native "test-ids". Content sections are state-dependent and
        // asserted by the state tests below instead.
        foreach (var id in new[] { "StatusText", "CollapseButton", "OpenSessionButton" })
        {
            _ = _ui.Find(id);
        }
    }

    [WinUiUiFact]
    public void Status_line_reflects_live_snapshot_and_healthy_state_is_silent()
    {
        // PanelWindow.Render replaces the construction-time "Agent Grid" text
        // with the aggregate summary on the first supervision snapshot —
        // proves live REST/WS wiring end to end. A healthy connection shows
        // no badge at all (ConnectionText exists only in the offline state).
        var rendered = Retry.WhileFalse(
            () => _ui.Find("StatusText").Name != "Agent Grid",
            TimeSpan.FromSeconds(20),
            interval: TimeSpan.FromMilliseconds(500)).Result;
        if (!rendered)
        {
            var shot = _ui.TryCaptureWindow("no-snapshot-rendered");
            Assert.Fail(
                $"StatusText never left its initial text (gateway {WinUiUi.GatewayUrl})" +
                (shot is null ? "" : $"; screenshot: {shot}"));
        }

        Assert.Null(_ui.Window.FindFirstDescendant(cf => cf.ByAutomationId("ConnectionText")));
    }

    [WinUiUiFact]
    public void Empty_sections_do_not_exist_in_the_tree()
    {
        // No pending items → no pending list, no reply entry, no empty-state
        // filler. (run-dev has no live agent, so nothing is ever pending.)
        _ = _ui.Find("StatusText");
        Assert.Null(_ui.Window.FindFirstDescendant(cf => cf.ByAutomationId("PendingList")));
        Assert.Null(_ui.Window.FindFirstDescendant(cf => cf.ByAutomationId("EngageBox")));
    }

    [WinUiUiFact]
    public void Session_rows_fill_the_shell_content_width_when_present()
    {
        // Full-width rows make the content define the island width rather than
        // leaving short intrinsic-width chips inside an enlarged shell.
        _ = _ui.Find("StatusText");
        var chips = Retry.WhileEmpty(
            () => _ui.Window.FindAllDescendants(cf => cf.ByAutomationId("SessionChip")),
            TimeSpan.FromSeconds(10),
            interval: TimeSpan.FromMilliseconds(500)).Result ?? [];
        if (chips.Length > 0)
        {
            var windowRect = _ui.Window.BoundingRectangle;
            var stripRect = _ui.Find("SessionRow").BoundingRectangle;
            Assert.True(stripRect.Left >= windowRect.Left && stripRect.Right <= windowRect.Right,
                "session strip overflows the shell content bounds");
            Assert.All(chips, chip =>
            {
                Assert.True(chip.BoundingRectangle.Width >= stripRect.Width - 2,
                    $"session row width {chip.BoundingRectangle.Width} does not fill strip {stripRect.Width}");
                Assert.NotNull(chip.FindFirstDescendant(
                    cf => cf.ByAutomationId("SessionStatusIcon")));
            });
        }
        if (chips.Length < 2)
            return;

        var rects = chips
            .Select(c => c.BoundingRectangle)
            .Where(r => r.Width > 0 && r.Height > 0)
            .ToList();
        Assert.True(rects.Count >= 2,
            $"{chips.Length} rows in tree but only {rects.Count} laid out");
        Assert.Single(rects.Select(r => r.Left).Distinct());
        Assert.Equal(rects.Count, rects.Select(r => r.Top).Distinct().Count());
    }

    [WinUiUiFact]
    public void Island_collapses_to_compact_bar_and_expands_back()
    {
        // Vibe-island morph: CollapseButton → compact notch bar (expanded tree
        // leaves UIA); CompactBar → expanded panel returns. UI-local state only,
        // no session mutation (safe against a live run-dev).
        _ui.Find("CollapseButton").AsButton().Invoke();
        _ui.Find("CompactBar");
        _ui.Find("CompactStatusText");

        var expandedGone = Retry.WhileFalse(
            () => _ui.Window.FindFirstDescendant(cf => cf.ByAutomationId("StatusText")) is null,
            TimeSpan.FromSeconds(10),
            interval: TimeSpan.FromMilliseconds(250)).Result;
        Assert.True(expandedGone, "StatusText still in UIA tree after collapse");

        // Restore the expanded panel — later tests assert against its tree.
        _ui.Find("CompactBar").AsButton().Invoke();
        _ = _ui.Find("StatusText");
    }
}
