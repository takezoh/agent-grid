using AgentGrid.Shell.Composition;
using AgentGrid.Shell.Core.Health;
using AgentGrid.Shell.WinUI.Panel;
using H.NotifyIcon;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Controls;

namespace AgentGrid.Shell.WinUI.Tray;

/// <summary>
/// Tray icon bound to DaemonSupervisor health (contract-daemon-health-toast-budget:
/// tray only — no health toasts). Menu structurally separates Quit vs Stop-daemon.
/// </summary>
public sealed class TrayIconController : IDisposable
{
    private readonly ShellCompositionRoot _root;
    private readonly PanelWindow _panel;
    private readonly TaskbarIcon _icon;

    public TrayIconController(ShellCompositionRoot root, PanelWindow panel, Action quit)
    {
        _root = root;
        _panel = panel;

        var menu = new MenuFlyout();
        menu.Items.Add(MakeItem("Show panel", (_, _) => _panel.ToggleGlance()));
        menu.Items.Add(MakeItem("Restart daemon", async (_, _) =>
        {
            await _root.Menu.OnRestartDaemonAsync();
        }));
        menu.Items.Add(MakeItem("Stop daemon", async (_, _) =>
        {
            await _root.Menu.OnStopDaemonAsync();
        }));
        menu.Items.Add(new MenuFlyoutSeparator());
        // Quit MUST NOT call StopDaemon — ShellMenuHandlers.OnQuit enforces this.
        menu.Items.Add(MakeItem("Quit", (_, _) =>
        {
            _root.Menu.OnQuit();
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
        _root.Supervisor.SnapshotChanged += snap => Apply(TrayAppearanceMapper.From(snap));
    }

    public void Dispose() => _icon.Dispose();

    private void Apply(TrayAppearance appearance) => _icon.ToolTipText = appearance.Tooltip;

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
