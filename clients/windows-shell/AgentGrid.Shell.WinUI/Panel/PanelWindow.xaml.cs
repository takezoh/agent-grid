using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.DeepLinkRouter;
using AgentGrid.Shell.Core.SupervisionState;
using AgentGrid.Shell.Core.SessionIdentity;
using AgentGrid.Shell.Core.Configuration;
using AgentGrid.Shell.Platform.Interop;
using Microsoft.UI.Composition;
using Microsoft.UI.Dispatching;
using Microsoft.UI.Windowing;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;
using Microsoft.UI.Xaml.Hosting;
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
    // Sizing lives in IslandMetrics (DIP-authored, DPI-scaled at render time).
    private const double DragThresholdDips = 4;

    private readonly ShellFleet _root;
    private readonly IWin32InteropService _win32;
    private bool _flyoutOpen;
    private bool _noActivate = true;
    private IslandState _island = IslandState.Initial;
    private DispatcherQueueTimer? _morphTimer;
    private double _appliedScale = 1.0;
    private bool _syncQueued;

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
        MoveResizeTop(CurrentModeSize());

        _root.SnapshotChanged += snap =>
            DispatcherQueue.TryEnqueue(() => Render(PanelGlanceView.From(snap)));
        Render(_root.CurrentGlance());

        // Apply NOACTIVATE after HWND exists. Sizing correction cannot ride on
        // Activated alone: a NOACTIVATE window may never activate, so the
        // 1.0-scale construction guess would stick. Loaded fires regardless of
        // activation and is the earliest point the rasterization scale is
        // real; XamlRoot.Changed catches later DPI changes.
        Activated += (_, _) =>
        {
            var hwnd = WindowNative.GetWindowHandle(this);
            if (_noActivate)
                _win32.SetNoActivate(hwnd, noActivate: true);
            ApplyScaleIfChanged();
        };
        RootHost.Loaded += (_, _) =>
        {
            ApplyScaleIfChanged();
            // Re-measure once real fonts/items are in the tree.
            ScheduleExpandedSizeSync();
            // Changed also fires on every resize (each morph tick) — only
            // re-apply on an actual scale change or it fights the animation.
            if (RootHost.XamlRoot is { } xamlRoot)
                xamlRoot.Changed += (_, _) => ApplyScaleIfChanged();
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
            "system" => ElementTheme.Default,
            _ => ElementTheme.Dark,
        };
        if (appearance.Density != "comfortable" || appearance.FontScale != 1.0)
        {
            RootHost.Resources["ControlContentThemeFontSize"] = 14 * appearance.FontScale;
            RootHost.Resources["TextControlThemeFontSize"] = 14 * appearance.FontScale;
            RootHost.Resources["ControlContentThemeFontSizeSmall"] =
                (appearance.Density == "compact" ? 11 : 12) * appearance.FontScale;
        }
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
        // Vibe Island: the headline is the same aggregate the compact pill
        // shows; sections exist only when they have content (PanelSections).
        var compact = PanelPresentation.Compact(view);
        var sections = PanelPresentation.Sections(view);
        StatusText.Text = compact.Text;
        OfflineBadge.Visibility = sections.ShowOffline ? Visibility.Visible : Visibility.Collapsed;

        SessionList.ItemsSource = view.Sessions.Select(SessionChipVm.From).ToList();
        SessionRow.Visibility = sections.ShowSessions ? Visibility.Visible : Visibility.Collapsed;
        PendingList.ItemsSource = view.Pending.Select(PanelItemVm.From).ToList();
        PendingList.Visibility = sections.ShowPending ? Visibility.Visible : Visibility.Collapsed;
        if (!sections.ShowReply)
            ExitEngageIfNeeded();
        EngageRow.Visibility = sections.ShowReply ? Visibility.Visible : Visibility.Collapsed;

        var notice = view.Notices.LastOrDefault();
        NoticeText.Text = notice?.Headline ?? string.Empty;
        NoticeText.Visibility = sections.ShowNotice ? Visibility.Visible : Visibility.Collapsed;

        CompactStatusText.Text = compact.Text;
        CompactDot.Fill = PanelBrushes.ForToken(compact.AccentToken);
        CompactPendingText.Text = view.PendingCount.ToString();
        CompactPendingBadge.Visibility =
            view.PendingCount > 0 ? Visibility.Visible : Visibility.Collapsed;

        ApplyIsland(IslandStateMachine.OnSnapshot(_island, view.PendingCount));
        // Content changed → the expanded island re-hugs it (Vibe Island UX).
        ScheduleExpandedSizeSync();
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
        }
        else
        {
            CompactBar.Visibility = Visibility.Collapsed;
            ExpandedRoot.Visibility = Visibility.Visible;
        }
        MorphTo(CurrentModeSize());
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

    private void OnRunningStatusIconLoaded(object sender, RoutedEventArgs e)
    {
        if (sender is not FrameworkElement icon || icon.Visibility != Visibility.Visible)
            return;
        var visual = ElementCompositionPreview.GetElementVisual(icon);
        visual.CenterPoint = new System.Numerics.Vector3(7, 7, 0);
        var rotation = visual.Compositor.CreateScalarKeyFrameAnimation();
        rotation.InsertKeyFrame(1, 360);
        rotation.Duration = TimeSpan.FromMilliseconds(800);
        rotation.IterationBehavior = AnimationIterationBehavior.Forever;
        visual.StartAnimation(nameof(visual.RotationAngleInDegrees), rotation);
    }

    private void OnWaitingStatusDotLoaded(object sender, RoutedEventArgs e)
    {
        if (sender is not FrameworkElement dot || dot.Visibility != Visibility.Visible)
            return;
        var visual = ElementCompositionPreview.GetElementVisual(dot);
        ElementCompositionPreview.SetIsTranslationEnabled(dot, true);
        var compositor = visual.Compositor;
        var easing = compositor.CreateCubicBezierEasingFunction(
            new System.Numerics.Vector2(0.42f, 0),
            new System.Numerics.Vector2(0.58f, 1));
        var offset = compositor.CreateScalarKeyFrameAnimation();
        offset.InsertKeyFrame(0, 0);
        offset.InsertKeyFrame(0.3f, -1.46f, easing);
        offset.InsertKeyFrame(0.6f, 0, easing);
        offset.InsertKeyFrame(1, 0);
        offset.Duration = TimeSpan.FromMilliseconds(1200);
        offset.DelayTime = TimeSpan.FromMilliseconds(
            int.TryParse(dot.Tag?.ToString(), out var delay) ? delay : 0);
        offset.IterationBehavior = AnimationIterationBehavior.Forever;
        visual.StartAnimation("Translation.Y", offset);

        var opacity = compositor.CreateScalarKeyFrameAnimation();
        opacity.InsertKeyFrame(0, 0.35f);
        opacity.InsertKeyFrame(0.3f, 1, easing);
        opacity.InsertKeyFrame(0.6f, 0.35f, easing);
        opacity.InsertKeyFrame(1, 0.35f);
        opacity.Duration = offset.Duration;
        opacity.DelayTime = offset.DelayTime;
        opacity.IterationBehavior = AnimationIterationBehavior.Forever;
        visual.StartAnimation(nameof(visual.Opacity), opacity);
    }

    private void OnIdleStatusIconLoaded(object sender, RoutedEventArgs e)
    {
        if (sender is not FrameworkElement icon || icon.Visibility != Visibility.Visible)
            return;
        var visual = ElementCompositionPreview.GetElementVisual(icon);
        var opacity = visual.Compositor.CreateScalarKeyFrameAnimation();
        opacity.InsertKeyFrame(0, 0.55f);
        opacity.InsertKeyFrame(0.5f, 0.95f);
        opacity.InsertKeyFrame(1, 0.55f);
        opacity.Duration = TimeSpan.FromMilliseconds(2600);
        opacity.IterationBehavior = AnimationIterationBehavior.Forever;
        visual.StartAnimation(nameof(visual.Opacity), opacity);
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

    private IslandSize CurrentModeSize() =>
        _island.Mode == IslandMode.Compact
            ? IslandMetrics.Compact(Scale())
            : ExpandedTargetSize();

    /// <summary>
    /// Measure the panel content at the max expanded width and unlimited
    /// height; IslandMetrics clamps the result into the expanded bounds.
    /// </summary>
    private IslandSize ExpandedTargetSize()
    {
        try
        {
            RootHost.Measure(new Windows.Foundation.Size(
                IslandMetrics.ExpandedMaxWidthDips, double.PositiveInfinity));
            var desired = RootHost.DesiredSize;
            return IslandMetrics.ExpandedFor(desired.Width, desired.Height, Scale());
        }
        catch
        {
            return IslandMetrics.ExpandedFor(0, 0, Scale()); // min bounds
        }
    }

    /// <summary>
    /// Coalesced content-size re-sync: measure after the current dispatcher
    /// work (item containers realize by then) and morph if the target moved.
    /// MorphTo no-ops when the size is already right, so repeat calls are
    /// cheap and cannot oscillate.
    /// </summary>
    private void ScheduleExpandedSizeSync()
    {
        if (_syncQueued)
            return;
        _syncQueued = true;
        DispatcherQueue.TryEnqueue(() =>
        {
            _syncQueued = false;
            if (_island.Mode == IslandMode.Expanded)
                MorphTo(ExpandedTargetSize());
        });
    }

    private void ApplyScaleIfChanged()
    {
        var scale = Scale();
        if (Math.Abs(scale - _appliedScale) < 0.001)
            return;
        _appliedScale = scale;
        MoveResizeTop(CurrentModeSize());
    }

    /// <summary>Eased notch morph: a few MoveAndResize steps around the anchor.</summary>
    private void MorphTo(IslandSize target)
    {
        // Fit up-front so the from==target no-op check compares the size that
        // will actually be applied (repeat syncs stay idempotent at the edge).
        try
        {
            var work = WorkArea();
            target = IslandLayout.Fit(target, work.Width, work.Height);
        }
        catch
        {
            /* headless */
        }

        var (targetWidth, targetHeight) = target;
        _morphTimer?.Stop();
        int fromW, fromH;
        try
        {
            fromW = AppWindow.Size.Width;
            fromH = AppWindow.Size.Height;
        }
        catch
        {
            MoveResizeTop(target);
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
    private void MoveResizeTop(IslandSize size) => MoveResizeTop(size.Width, size.Height);

    private void MoveResizeTop(int width, int height)
    {
        try
        {
            var work = WorkArea();
            // Fit before clamping: on a small/heavily-scaled display the
            // expanded island itself can be wider than the work area.
            (width, height) = IslandLayout.Fit(
                new IslandSize(width, height), work.Width, work.Height);
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

    /// <summary>Work area of the display the island actually occupies.</summary>
    private Windows.Graphics.RectInt32 WorkArea() =>
        Microsoft.UI.Windowing.DisplayArea
            .GetFromWindowId(AppWindow.Id, Microsoft.UI.Windowing.DisplayAreaFallback.Nearest)
            .WorkArea;

    /// <summary>Move to x (physical), clamped on-screen, glued to the top edge.</summary>
    private void MoveDragTo(int x)
    {
        try
        {
            var work = WorkArea();
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
