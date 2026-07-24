using FlaUI.Core.AutomationElements;
using FlaUI.Core.Tools;

namespace AgentGrid.Shell.WinUI.UiTests.E2E;

/// <summary>
/// T3 opt-in UI facts against the real WinUI panel via UIA (FlaUI).
/// Harness: clients/windows-shell/scripts/e2e.sh (WinUI stage builds the exe,
/// launch-smoke verifies startup, then this suite drives the live window).
/// </summary>
public sealed class PanelWindowUiE2ETests : IClassFixture<PanelUiSession>
{
    private readonly PanelUiSession _ui;

    public PanelWindowUiE2ETests(PanelUiSession ui) => _ui = ui;

    [WinUiUiFact]
    public void Panel_exposes_glance_elements_via_automation_ids()
    {
        // The AutomationId contract of PanelWindow.xaml — the native "test-ids".
        foreach (var id in new[]
                 {
                     "StatusText", "ConnectionText", "PendingHeader", "PendingList",
                     "EngageBox", "EngageSendButton", "OpenSessionButton",
                 })
        {
            _ = _ui.Find(id);
        }
    }

    [WinUiUiFact]
    public void Connection_text_reports_online_against_gateway()
    {
        // PanelWindow.Render: "online" once a supervision snapshot arrives
        // without ConnectionFailed — proves live REST/WS wiring end to end.
        var online = Retry.WhileFalse(
            () => _ui.Find("ConnectionText").Name == "online",
            TimeSpan.FromSeconds(20),
            interval: TimeSpan.FromMilliseconds(500)).Result;
        if (!online)
        {
            var shot = _ui.TryCaptureWindow("connection-not-online");
            Assert.Fail(
                $"ConnectionText stayed '{_ui.Find("ConnectionText").Name}' " +
                $"(gateway {WinUiUi.GatewayUrl})" +
                (shot is null ? "" : $"; screenshot: {shot}"));
        }
    }

    [WinUiUiFact]
    public void Engage_box_accepts_text_and_send_clears_it()
    {
        // With no pending question, OnEngageSend clears the box — deterministic
        // round-trip proving ValuePattern input + Invoke reach real handlers.
        var box = _ui.Find("EngageBox").AsTextBox();
        box.Text = "agent-grid ui-e2e probe";
        Assert.Equal("agent-grid ui-e2e probe", box.Text);

        _ui.Find("EngageSendButton").AsButton().Invoke();

        var cleared = Retry.WhileFalse(
            () => box.Text.Length == 0,
            TimeSpan.FromSeconds(10),
            interval: TimeSpan.FromMilliseconds(250)).Result;
        Assert.True(cleared, $"EngageBox not cleared after Send; text='{box.Text}'");
    }
}
