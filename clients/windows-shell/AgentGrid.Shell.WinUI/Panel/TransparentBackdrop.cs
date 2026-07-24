using System.Runtime.InteropServices;
using Microsoft.UI.Composition;
using Microsoft.UI.Xaml;
using Microsoft.UI.Xaml.Media;

namespace AgentGrid.Shell.WinUI.Panel;

/// <summary>
/// Fully transparent system backdrop: the HWND surface contributes nothing,
/// so the island's shape is exactly what RootHost paints (flat top glued to
/// the screen edge, rounded bottom like the macOS notch). Transparent areas
/// are not hit-testable, so the cut corners click through.
/// </summary>
internal sealed partial class TransparentBackdrop : SystemBackdrop
{
    // ICompositionSupportsSystemBackdrop wants a Windows.UI.Composition brush;
    // creating that Compositor needs a Windows.System.DispatcherQueue on this
    // thread (standard WASDK backdrop boilerplate).
    private static object? _dispatcherQueueController;
    private Windows.UI.Composition.Compositor? _compositor;

    protected override void OnTargetConnected(
        ICompositionSupportsSystemBackdrop connectedTarget, XamlRoot xamlRoot)
    {
        base.OnTargetConnected(connectedTarget, xamlRoot);
        EnsureSystemDispatcherQueue();
        _compositor ??= new Windows.UI.Composition.Compositor();
        connectedTarget.SystemBackdrop = _compositor.CreateColorBrush(
            Windows.UI.Color.FromArgb(0, 0, 0, 0));
    }

    protected override void OnTargetDisconnected(ICompositionSupportsSystemBackdrop disconnectedTarget)
    {
        disconnectedTarget.SystemBackdrop = null;
        base.OnTargetDisconnected(disconnectedTarget);
    }

    private static void EnsureSystemDispatcherQueue()
    {
        if (Windows.System.DispatcherQueue.GetForCurrentThread() is not null)
            return;
        if (_dispatcherQueueController is not null)
            return;

        var options = new DispatcherQueueOptions
        {
            Size = Marshal.SizeOf<DispatcherQueueOptions>(),
            ThreadType = 2, // DQTYPE_THREAD_CURRENT
            ApartmentType = 2, // DQTAT_COM_STA
        };
        object? controller = null;
        _ = CreateDispatcherQueueController(options, ref controller);
        _dispatcherQueueController = controller;
    }

    [StructLayout(LayoutKind.Sequential)]
    private struct DispatcherQueueOptions
    {
        public int Size;
        public int ThreadType;
        public int ApartmentType;
    }

    [DllImport("CoreMessaging.dll")]
    private static extern int CreateDispatcherQueueController(
        DispatcherQueueOptions options,
        [MarshalAs(UnmanagedType.IUnknown)] ref object? dispatcherQueueController);
}
