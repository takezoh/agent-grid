using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.WinUI.Panel;
using H.NotifyIcon;
using Microsoft.UI.Dispatching;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace AgentGrid.Shell.WinUI.Tray;

/// <summary>
/// Tray icon bound to DaemonSupervisor health (contract-daemon-health-toast-budget:
/// tray only — no health toasts). Menu structurally separates Quit vs Stop-daemon.
/// </summary>
public sealed class TrayIconController : IDisposable
{
    private readonly ShellFleet _root;
    private readonly PanelWindow _panel;
    private readonly TaskbarIcon _icon;
    private readonly DispatcherQueue _dispatcher;

    public TrayIconController(ShellFleet root, PanelWindow panel, Action quit)
    {
        _root = root;
        _panel = panel;
        // ShellFleet.HealthChanged (backed by DaemonSupervisorService.SnapshotChanged)
        // may fire on an arbitrary thread — capture the UI-thread dispatcher here
        // (construction always happens on the UI thread) so Apply can marshal.
        _dispatcher = DispatcherQueue.GetForCurrentThread();

        var menu = new MenuFlyout();
        menu.Items.Add(MakeItem("Show panel", (_, _) => _panel.ToggleGlance()));
        menu.Items.Add(MakeItem("Restart daemon", async (_, _) =>
        {
            await _root.RestartAllAsync();
        }));
        menu.Items.Add(MakeItem("Stop daemon", async (_, _) =>
        {
            await _root.StopAllAsync();
        }));
        menu.Items.Add(new MenuFlyoutSeparator());
        // Quit MUST NOT call StopDaemon — ShellMenuHandlers.OnQuit enforces this.
        menu.Items.Add(MakeItem("Quit", (_, _) =>
        {
            _root.Quit();
            quit();
        }));

        _icon = new TaskbarIcon
        {
            ToolTipText = "Agent Grid",
            ContextFlyout = menu,
        };
        _icon.ForceCreate(false);
        _icon.LeftClickCommand = new RelayCommand(() => _panel.ToggleGlance());

        Apply(_root.CurrentTrayAppearance());
        _root.HealthChanged += Apply;
    }

    public void Dispose() => _icon.Dispose();

    /// <summary>
    /// HealthChanged can fire on a thread-pool thread (any real async I/O in the
    /// daemon-health probe resumes the SnapshotChanged continuation off the UI
    /// thread) — marshal before touching the TaskbarIcon DependencyObject.
    /// </summary>
    private void Apply(TrayAppearance appearance)
    {
        if (_dispatcher.HasThreadAccess)
        {
            _icon.ToolTipText = appearance.Tooltip;
        }
        else
        {
            _dispatcher.TryEnqueue(() => _icon.ToolTipText = appearance.Tooltip);
        }
    }

    private static MenuFlyoutItem MakeItem(string text, RoutedEventHandler handler)
    {
        var item = new MenuFlyoutItem { Text = text };
        item.Click += handler;
        return item;
    }

    private sealed class RelayCommand : System.Windows.Input.ICommand
    {
        private readonly Action _execute;
        public RelayCommand(Action execute) => _execute = execute;
        public bool CanExecute(object? parameter) => true;
        public void Execute(object? parameter) => _execute();
#pragma warning disable CS0067
        public event EventHandler? CanExecuteChanged;
#pragma warning restore CS0067
    }
}
