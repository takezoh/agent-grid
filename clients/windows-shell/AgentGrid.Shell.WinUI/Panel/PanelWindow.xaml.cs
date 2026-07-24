using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DeepLinkRouter;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Core.SessionIdentity;
using AgentGrid.Shell.Core.Configuration;
using AgentGrid.Shell.Platform.Interop;
using Microsoft.UI.Dispatching;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Input;
using Microsoft.UI.Xaml.Media;
using WinRT.Interop;

namespace AgentGrid.Shell.WinUI.Panel;

/// <summary>
/// Dynamic-island glance panel (Vibe Island UX): lives as a compact notch bar
/// at top-center, auto-expands when an approval/question arrives, auto-collapses
/// when the queue drains (IslandStateMachine owns the transitions).
/// Starts with WS_EX_NOACTIVATE; engage entry removes it and arms
/// EngageFocusService for restore-on-exit.
/// </summary>
public sealed partial class PanelWindow : Window
{
    // Physical px; widths include the two 12-logical-px concave wings.
    private const int ExpandedWidth = 490;
    private const int ExpandedHeight = 540;
    private const int CompactWidth = 370;
    private const int CompactHeight = 48;
    private const double DragThresholdDips = 4;

    private readonly ShellFleet _root;
    private readonly IWin32InteropService _win32;
    private bool _flyoutOpen;
    private bool _noActivate = true;
    private IslandState _island = IslandState.Initial;
    private DispatcherQueueTimer? _morphTimer;

    // Horizontal drag anchor: center X in physical px; null = screen center.
    private int? _centerX;
    private bool _dragPressed;
    private bool _dragging;
    private bool _suppressClick;
    private uint _dragPointerId;
    private double _dragStartScreenX;
    private int _dragStartWindowX;

    public PanelWindow(ShellFleet root)
    {
        _root = root;
        _win32 = root.Win32;
        InitializeComponent();
        ApplyAppearance(root.Config.Appearance);
        ConfigureIslandChrome();
        AttachDragSurface(CompactBar);
        AttachDragSurface(HeaderBar);
        MoveResizeTop(ExpandedWidth, ExpandedHeight);

        _root.SnapshotChanged += snap =>
            DispatcherQueue.TryEnqueue(() => Render(PanelGlanceView.From(snap)));
        Render(_root.CurrentGlance());

        // Apply NOACTIVATE after HWND exists; re-clip the notch region once
        // the XAML root is live (rasterization scale is known by then).
        Activated += (_, _) =>
        {
            var hwnd = WindowNative.GetWindowHandle(this);
            if (_noActivate)
                _win32.SetNoActivate(hwnd, noActivate: true);
            ApplyNotchRegionForCurrentSize();
        };
    }

    public bool IsFlyoutOpen => _flyoutOpen;
    public nint Hwnd => WindowNative.GetWindowHandle(this);

    private void ApplyAppearance(AppearanceConfig appearance)
    {
        RootHost.RequestedTheme = appearance.Theme switch
        {
            "light" => ElementTheme.Light,
            "dark" => ElementTheme.Dark,
            _ => ElementTheme.Default,
        };
        RootHost.FontSize = 14 * appearance.FontScale;
        RootHost.Resources["ControlContentThemeFontSize"] = 14 * appearance.FontScale;
        RootHost.Resources["TextControlThemeFontSize"] = 14 * appearance.FontScale;
        RootHost.Resources["ControlContentThemeFontSizeSmall"] =
            (appearance.Density == "compact" ? 11 : 12) * appearance.FontScale;
    }

    public void ShowGlance()
    {
        _flyoutOpen = true;
        EnsureNoActivate(true);
        AppWindow.Show();
        Interop.WindowChrome.StripClassicEdges(Hwnd);
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
                ApplyIsland(IslandStateMachine.OnUserExpand(_island));
                ShowGlance();
                break;
            case RouteKind.OpenWorkspaceSession when d.ServerId is not null && d.Id is not null:
                await _root.OpenSessionAsync(new ServerSessionId(d.ServerId, d.Id));
                break;
            case RouteKind.JumpBack when d.Id is not null:
                _ = _root.JumpBack.Jump(new Platform.JumpBack.JumpBackTarget(
                    null, "WindowsTerminal", null, null));
                break;
        }
    }

    /// <summary>
    /// Notch chrome: frameless, always on top, DWM rounding off, transparent
    /// backdrop — RootHost paints the flat-top/round-bottom slab, so the
    /// island reads as attached to the screen edge (Vibe Island look).
    /// </summary>
    private void ConfigureIslandChrome()
    {
        try
        {
            SystemBackdrop = new TransparentBackdrop();
        }
        catch
        {
            /* backdrop unsupported — RootHost tint still gives the dark island */
        }

        if (AppWindow.Presenter is OverlappedPresenter presenter)
        {
            presenter.IsAlwaysOnTop = true;
            presenter.IsMaximizable = false;
            presenter.IsMinimizable = false;
            presenter.IsResizable = false;
            presenter.SetBorderAndTitleBar(false, false);
        }

        Interop.WindowChrome.DisableDwmRoundedCorners(Hwnd);
        Interop.WindowChrome.DisableDwmBorder(Hwnd);
        Interop.WindowChrome.StripClassicEdges(Hwnd);
    }

    private void Render(PanelGlanceView view)
    {
        StatusText.Text = view.StatusLine;
        ConnectionText.Text = view.ConnectionFailed ? "offline" : "online";
        ConnectionDot.Fill = view.ConnectionFailed ? PanelBrushes.Danger : PanelBrushes.Success;
        PendingHeader.Text = view.PendingCount == 0 ? "No pending items" : $"{view.PendingCount} pending";
        SessionList.ItemsSource = view.Sessions.Select(SessionChipVm.From).ToList();
        PendingList.ItemsSource = view.Pending.Select(PanelItemVm.From).ToList();
        EmptyState.Visibility = view.PendingCount == 0 ? Visibility.Visible : Visibility.Collapsed;
        ShortcutHint.Visibility = view.Pending.Any(p => p.Kind == "approval")
            ? Visibility.Visible
            : Visibility.Collapsed;

        var notice = view.Notices.LastOrDefault();
        NoticeText.Text = notice?.Headline ?? string.Empty;
        NoticeText.Visibility = notice is null ? Visibility.Collapsed : Visibility.Visible;

        var compact = PanelPresentation.Compact(view);
        CompactStatusText.Text = compact.Text;
        CompactDot.Fill = PanelBrushes.ForToken(compact.AccentToken);
        CompactPendingText.Text = view.PendingCount.ToString();
        CompactPendingBadge.Visibility =
            view.PendingCount > 0 ? Visibility.Visible : Visibility.Collapsed;

        ApplyIsland(IslandStateMachine.OnSnapshot(_island, view.PendingCount));
    }

    /// <summary>Applies a state-machine result: swap layers + morph the window.</summary>
    private void ApplyIsland(IslandState next)
    {
        var modeChanged = next.Mode != _island.Mode;
        _island = next;
        if (!modeChanged)
            return;

        if (next.Mode == IslandMode.Compact)
        {
            ExitEngageIfNeeded();
            CompactBar.Visibility = Visibility.Visible;
            ExpandedRoot.Visibility = Visibility.Collapsed;
            MorphTo(CompactWidth, CompactHeight);
        }
        else
        {
            CompactBar.Visibility = Visibility.Collapsed;
            ExpandedRoot.Visibility = Visibility.Visible;
            MorphTo(ExpandedWidth, ExpandedHeight);
        }
    }

    private void OnCompactBarClick(object sender, RoutedEventArgs e)
    {
        // A horizontal drag ends in a click on the same Button — swallow it.
        if (_suppressClick)
        {
            _suppressClick = false;
            return;
        }
        ApplyIsland(IslandStateMachine.OnUserExpand(_island));
    }

    private void OnCollapseClick(object sender, RoutedEventArgs e) =>
        ApplyIsland(IslandStateMachine.OnUserCollapse(_island));

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

    private async void OnAllowAccelerator(
        KeyboardAccelerator sender, KeyboardAcceleratorInvokedEventArgs args)
    {
        args.Handled = true;
        var item = FirstPendingApproval();
        if (item is not null)
            await SubmitApprovalAsync(item, "accept");
    }

    private async void OnDenyAccelerator(
        KeyboardAccelerator sender, KeyboardAcceleratorInvokedEventArgs args)
    {
        args.Handled = true;
        var item = FirstPendingApproval();
        if (item is not null)
            await SubmitApprovalAsync(item, "deny");
    }

    private void OnEscapeAccelerator(
        KeyboardAccelerator sender, KeyboardAcceleratorInvokedEventArgs args)
    {
        args.Handled = true;
        if (_island.Mode == IslandMode.Expanded)
        {
            ApplyIsland(IslandStateMachine.OnUserCollapse(_island));
            return;
        }
        HideGlance();
    }

    private PanelGlanceItem? FirstPendingApproval() =>
        _root.CurrentGlance().Pending.FirstOrDefault(p => p.Kind == "approval");

    private async Task SubmitApprovalAsync(PanelGlanceItem item, string decision)
    {
        await _root.SubmitApprovalAsync(
            item.ServerId, item.ItemId, item.SessionId, decision, item.Headline, item.ExpiresAt);
        ExitEngageIfNeeded();
    }

    private void OnEngageGotFocus(object sender, RoutedEventArgs e)
    {
        // Engage: allow activation + capture prior foreground for restore.
        EnsureNoActivate(false);
        _root.Engage.EnterEngage();
        Activate();
    }

    private async void OnEngageKeyDown(object sender, KeyRoutedEventArgs e)
    {
        if (e.Key != Windows.System.VirtualKey.Enter)
            return;
        e.Handled = true;
        await SubmitEngageAsync();
    }

    private async void OnEngageSend(object sender, RoutedEventArgs e) =>
        await SubmitEngageAsync();

    private async Task SubmitEngageAsync()
    {
        var answer = EngageBox.Text?.Trim() ?? "";
        if (string.IsNullOrEmpty(answer))
        {
            ExitEngageIfNeeded();
            return;
        }

        var item = (PendingList.SelectedItem as PanelItemVm)?.Item
                   ?? _root.CurrentGlance().Pending.FirstOrDefault(p => p.Kind == "question");
        if (item is null || item.Kind != "question")
        {
            ExitEngageIfNeeded();
            EngageBox.Text = string.Empty;
            return;
        }

        await _root.SubmitQuestionAsync(
            item.ServerId, item.ItemId, item.SessionId, answer, item.Headline);
        EngageBox.Text = string.Empty;
        ExitEngageIfNeeded();
    }

    private async void OnSessionChipClick(object sender, RoutedEventArgs e)
    {
        if ((sender as FrameworkElement)?.Tag is not SessionChipVm chip)
            return;
        await _root.OpenSessionAsync(
            new ServerSessionId(chip.Session.ServerId, chip.Session.SessionId));
    }

    private async void OnOpenSession(object sender, RoutedEventArgs e)
    {
        var sessions = _root.CurrentGlance().Sessions;
        var session = sessions.FirstOrDefault();
        if (session is not null)
            await _root.OpenSessionAsync(new ServerSessionId(session.ServerId, session.SessionId));
        else if (PendingList.SelectedItem is PanelItemVm vm)
            await _root.OpenSessionAsync(
                new ServerSessionId(vm.Item.ServerId, vm.Item.SessionId));
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

    /// <summary>Eased notch morph: a few MoveAndResize steps around the anchor.</summary>
    private void MorphTo(int targetWidth, int targetHeight)
    {
        _morphTimer?.Stop();
        int fromW, fromH;
        try
        {
            fromW = AppWindow.Size.Width;
            fromH = AppWindow.Size.Height;
        }
        catch
        {
            MoveResizeTop(targetWidth, targetHeight);
            return;
        }

        if (fromW == targetWidth && fromH == targetHeight)
            return;

        const int steps = 8;
        var i = 0;
        var timer = DispatcherQueue.CreateTimer();
        timer.Interval = TimeSpan.FromMilliseconds(15);
        timer.Tick += (t, _) =>
        {
            i++;
            var progress = i / (double)steps;
            var eased = 1 - Math.Pow(1 - progress, 3);
            var w = (int)Math.Round(fromW + (targetWidth - fromW) * eased);
            var h = (int)Math.Round(fromH + (targetHeight - fromH) * eased);
            MoveResizeTop(w, h);
            if (i >= steps)
                t.Stop();
        };
        _morphTimer = timer;
        timer.Start();
    }

    /// <summary>
    /// Fit to the top screen edge, horizontally anchored at the drag center
    /// (default: screen center) and clamped inside the work area.
    /// </summary>
    private void MoveResizeTop(int width, int height)
    {
        try
        {
            var work = Microsoft.UI.Windowing.DisplayArea.Primary.WorkArea;
            var center = _centerX ?? work.X + work.Width / 2;
            var x = IslandLayout.ClampedX(center, width, work.X, work.Width);
            AppWindow.MoveAndResize(new Windows.Graphics.RectInt32(
                x, work.Y, width, height));
        }
        catch
        {
            // Headless / no display in CI: at least apply the size.
            try
            {
                AppWindow.Resize(new Windows.Graphics.SizeInt32(width, height));
            }
            catch
            {
                /* no HWND yet */
            }
        }

        // AppWindow units are physical px, so the region size matches directly.
        Interop.WindowChrome.ApplyNotchRegion(
            Hwnd, width, height, NotchRadiusPhysical(), WingRadiusPhysical());
    }

    /// <summary>Re-clip using the actual current window size (post-activation).</summary>
    private void ApplyNotchRegionForCurrentSize()
    {
        try
        {
            Interop.WindowChrome.ApplyNotchRegion(
                Hwnd, AppWindow.Size.Width, AppWindow.Size.Height,
                NotchRadiusPhysical(), WingRadiusPhysical());
        }
        catch
        {
            /* headless */
        }
    }

    private int NotchRadiusPhysical() => (int)Math.Round(20 * Scale());

    private int WingRadiusPhysical() => (int)Math.Round(12 * Scale());

    // ---- Horizontal drag (CompactBar + expanded HeaderBar) ----

    private void AttachDragSurface(UIElement surface)
    {
        // handledEventsToo: Buttons mark presses handled; we still track them.
        surface.AddHandler(UIElement.PointerPressedEvent,
            new PointerEventHandler(OnDragSurfacePressed), handledEventsToo: true);
        surface.AddHandler(UIElement.PointerMovedEvent,
            new PointerEventHandler(OnDragSurfaceMoved), handledEventsToo: true);
        surface.AddHandler(UIElement.PointerReleasedEvent,
            new PointerEventHandler(OnDragSurfaceReleased), handledEventsToo: true);
        surface.AddHandler(UIElement.PointerCaptureLostEvent,
            new PointerEventHandler(OnDragSurfaceCaptureLost), handledEventsToo: true);
    }

    private void OnDragSurfacePressed(object sender, PointerRoutedEventArgs e)
    {
        _suppressClick = false;
        _dragging = false;
        _dragPointerId = e.Pointer.PointerId;
        try
        {
            _dragStartWindowX = AppWindow.Position.X;
        }
        catch
        {
            return; // headless / no HWND
        }
        if (!Interop.WindowChrome.TryGetCursorScreenX(out var cursorX))
            return;
        _dragStartScreenX = cursorX;
        _dragPressed = true;
    }

    private void OnDragSurfaceMoved(object sender, PointerRoutedEventArgs e)
    {
        if (!_dragPressed || e.Pointer.PointerId != _dragPointerId)
            return;
        if (!e.GetCurrentPoint(null).Properties.IsLeftButtonPressed)
            return;
        // The pointer event only wakes us up; position comes from the live
        // cursor (physical screen px), immune to our own window movement.
        if (!Interop.WindowChrome.TryGetCursorScreenX(out var cursorX))
            return;

        var delta = cursorX - _dragStartScreenX;
        if (!_dragging)
        {
            if (Math.Abs(delta) < DragThresholdDips * Scale())
                return;
            // Steal capture (cancels a pending child-button click); horizontal
            // drag begins only past the threshold so plain clicks stay clicks.
            // Suppress must be armed HERE: Button.Click is raised from the
            // control's class handler on release, before our bubbled handler.
            _dragging = true;
            _suppressClick = true;
            ((UIElement)sender).CapturePointer(e.Pointer);
        }

        MoveDragTo(_dragStartWindowX + (int)Math.Round(delta));
    }

    private void OnDragSurfaceReleased(object sender, PointerRoutedEventArgs e)
    {
        if (e.Pointer.PointerId != _dragPointerId)
            return;
        _dragPressed = false;
        _dragging = false;
        try
        {
            ((UIElement)sender).ReleasePointerCapture(e.Pointer);
        }
        catch
        {
            /* capture already gone */
        }
    }

    private void OnDragSurfaceCaptureLost(object sender, PointerRoutedEventArgs e)
    {
        if (e.Pointer.PointerId != _dragPointerId)
            return;
        _dragPressed = false;
        _dragging = false;
    }

    /// <summary>Move to x (physical), clamped on-screen, glued to the top edge.</summary>
    private void MoveDragTo(int x)
    {
        try
        {
            var work = Microsoft.UI.Windowing.DisplayArea.Primary.WorkArea;
            var width = AppWindow.Size.Width;
            var clamped = IslandLayout.ClampLeft(x, width, work.X, work.Width);
            AppWindow.Move(new Windows.Graphics.PointInt32(clamped, work.Y));
            _centerX = clamped + width / 2;
        }
        catch
        {
            /* headless */
        }
    }

    private double Scale() => RootHost.XamlRoot?.RasterizationScale ?? 1.0;
}
