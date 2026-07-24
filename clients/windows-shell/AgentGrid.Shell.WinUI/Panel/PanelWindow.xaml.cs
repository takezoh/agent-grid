using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DeepLinkRouter;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Platform.Interop;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using WinRT.Interop;

namespace AgentGrid.Shell.WinUI.Panel;

/// <summary>
/// Always-on glance panel. Starts with WS_EX_NOACTIVATE; engage entry removes it
/// and arms EngageFocusService for restore-on-exit.
/// </summary>
public sealed partial class PanelWindow : Window
{
    private readonly ShellCompositionRoot _root;
    private readonly IWin32InteropService _win32;
    private bool _flyoutOpen;
    private bool _noActivate = true;

    public PanelWindow(ShellCompositionRoot root)
    {
        _root = root;
        _win32 = root.Win32;
        InitializeComponent();
        ExtendsContentIntoTitleBar = true;
        AppWindow.Resize(new Windows.Graphics.SizeInt32(420, 520));
        PositionTopCenter();

        _root.Supervision.SnapshotChanged += snap =>
            DispatcherQueue.TryEnqueue(() => Render(PanelGlanceView.From(snap)));
        Render(_root.CurrentGlance());

        // Apply NOACTIVATE after HWND exists.
        Activated += (_, _) =>
        {
            var hwnd = WindowNative.GetWindowHandle(this);
            if (_noActivate)
                _win32.SetNoActivate(hwnd, noActivate: true);
        };
    }

    public bool IsFlyoutOpen => _flyoutOpen;
    public nint Hwnd => WindowNative.GetWindowHandle(this);

    public void ShowGlance()
    {
        _flyoutOpen = true;
        EnsureNoActivate(true);
        AppWindow.Show();
        // First launch: activate once so the user sees the panel. Subsequent
        // tray toggles keep NOACTIVATE so we do not steal focus mid-work.
        try
        {
            Activate();
        }
        catch
        {
            /* headless / race during HWND create */
        }
    }

    public void HideGlance()
    {
        _flyoutOpen = false;
        AppWindow.Hide();
    }

    public void ToggleGlance()
    {
        if (_flyoutOpen) HideGlance();
        else ShowGlance();
    }

    public async Task HandleDeepLinkAsync(string uri)
    {
        var d = DeepLinkRouter.Route(uri);
        switch (d.Kind)
        {
            case RouteKind.PanelFocusItem:
                ShowGlance();
                break;
            case RouteKind.OpenWorkspaceSession when d.Id is not null:
                await _root.WorkspaceLauncher.OpenSessionAsync(d.Id);
                break;
            case RouteKind.JumpBack when d.Id is not null:
                _ = _root.JumpBack.Jump(new Platform.JumpBack.JumpBackTarget(
                    null, "WindowsTerminal", null, null));
                break;
        }
    }

    private void Render(PanelGlanceView view)
    {
        StatusText.Text = view.StatusLine;
        ConnectionText.Text = view.ConnectionFailed ? "offline" : "online";
        PendingHeader.Text = view.PendingCount == 0 ? "No pending items" : $"{view.PendingCount} pending";
        PendingList.ItemsSource = view.Pending.ToList();
    }

    private async void OnApproveClick(object sender, RoutedEventArgs e)
    {
        if ((sender as FrameworkElement)?.Tag is not PanelGlanceItem item || item.Kind != "approval")
            return;
        await SubmitApprovalAsync(item, "accept");
    }

    private async void OnDenyClick(object sender, RoutedEventArgs e)
    {
        if ((sender as FrameworkElement)?.Tag is not PanelGlanceItem item || item.Kind != "approval")
            return;
        await SubmitApprovalAsync(item, "deny");
    }

    private async Task SubmitApprovalAsync(PanelGlanceItem item, string decision)
    {
        await _root.Supervision.SubmitApprovalAsync(
            item.ItemId, item.SessionId, decision, item.Headline, item.ExpiresAt);
        ExitEngageIfNeeded();
    }

    private void OnEngageGotFocus(object sender, RoutedEventArgs e)
    {
        // Engage: allow activation + capture prior foreground for restore.
        EnsureNoActivate(false);
        _root.Engage.EnterEngage();
        Activate();
    }

    private async void OnEngageSend(object sender, RoutedEventArgs e)
    {
        var answer = EngageBox.Text?.Trim() ?? "";
        if (string.IsNullOrEmpty(answer))
        {
            ExitEngageIfNeeded();
            return;
        }

        var item = PendingList.SelectedItem as PanelGlanceItem
                   ?? _root.CurrentGlance().Pending.FirstOrDefault(p => p.Kind == "question");
        if (item is null || item.Kind != "question")
        {
            ExitEngageIfNeeded();
            EngageBox.Text = string.Empty;
            return;
        }

        await _root.Supervision.SubmitQuestionAsync(
            item.ItemId, item.SessionId, answer, item.Headline);
        EngageBox.Text = string.Empty;
        ExitEngageIfNeeded();
    }

    private async void OnOpenSession(object sender, RoutedEventArgs e)
    {
        var sessions = _root.CurrentGlance().Sessions;
        var id = sessions.FirstOrDefault()?.SessionId;
        if (id is null && PendingList.SelectedItem is PanelGlanceItem item)
            id = item.SessionId;
        if (id is not null)
            await _root.WorkspaceLauncher.OpenSessionAsync(id);
    }

    private void ExitEngageIfNeeded()
    {
        if (!_root.Engage.IsArmed) return;
        var hwnd = Hwnd;
        _root.Engage.ExitEngage(shellHwnd: hwnd);
        EnsureNoActivate(true);
    }

    private void EnsureNoActivate(bool noActivate)
    {
        _noActivate = noActivate;
        var hwnd = Hwnd;
        if (hwnd != 0)
            _win32.SetNoActivate(hwnd, noActivate);
    }

    private void PositionTopCenter()
    {
        try
        {
            var display = Microsoft.UI.Windowing.DisplayArea.Primary;
            var work = display.WorkArea;
            var w = 420;
            var x = work.X + (work.Width - w) / 2;
            var y = work.Y + 12;
            AppWindow.Move(new Windows.Graphics.PointInt32(x, y));
        }
        catch
        {
            /* headless / no display in CI */
        }
    }
}
