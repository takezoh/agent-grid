using AgentGrid.Shell.Platform.Interop;
using Microsoft.UI.Xaml;
using WinRT.Interop;

namespace AgentGrid.Shell.WinUI.Interop;

/// <summary>Helpers for panel HWND chrome (NOACTIVATE, etc.).</summary>
public static class WindowChrome
{
    public static nint GetHwnd(Window window) => WindowNative.GetWindowHandle(window);

    public static void ApplyNoActivate(IWin32InteropService win32, Window window, bool noActivate)
    {
        var hwnd = GetHwnd(window);
        if (hwnd != 0)
            win32.SetNoActivate(hwnd, noActivate);
    }
}
