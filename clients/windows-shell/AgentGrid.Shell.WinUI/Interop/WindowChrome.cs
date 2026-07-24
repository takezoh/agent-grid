using System.Runtime.InteropServices;
using AgentGrid.Shell.Platform.Interop;
using Microsoft.UI.Xaml;
using WinRT.Interop;

namespace AgentGrid.Shell.WinUI.Interop;

/// <summary>Helpers for panel HWND chrome (NOACTIVATE, notch corners, etc.).</summary>
public static class WindowChrome
{
    private const uint DwmwaWindowCornerPreference = 33;
    private const uint DwmwcpDoNotRound = 1;
    private const uint DwmwaBorderColor = 34;
    private const uint DwmwaColorNone = 0xFFFFFFFE;
    private const int RgnOr = 2;
    private const int RgnDiff = 4;

    private const int GwlStyle = -16;
    private const int GwlExStyle = -20;
    private const long WsFrameBits = 0x00C40000; // WS_BORDER | WS_DLGFRAME | WS_THICKFRAME
    private const long WsExEdgeBits = 0x00020300; // WINDOWEDGE | CLIENTEDGE | STATICEDGE
    // FRAMECHANGED | NOSIZE | NOMOVE | NOZORDER | NOACTIVATE
    private const uint SwpFrameChanged = 0x0020 | 0x0001 | 0x0002 | 0x0004 | 0x0010;

    public static nint GetHwnd(Window window) => WindowNative.GetWindowHandle(window);

    public static void ApplyNoActivate(IWin32InteropService win32, Window window, bool noActivate)
    {
        var hwnd = GetHwnd(window);
        if (hwnd != 0)
            win32.SetNoActivate(hwnd, noActivate);
    }

    /// <summary>
    /// Kill Win11 DWM corner rounding so the frameless island sits flush
    /// against the top screen edge (the XAML root rounds only the bottom).
    /// Best-effort: unsupported (Win10) is fine — it never rounds there.
    /// </summary>
    public static void DisableDwmRoundedCorners(nint hwnd)
    {
        if (hwnd == 0)
            return;
        var pref = DwmwcpDoNotRound;
        _ = DwmSetWindowAttribute(hwnd, DwmwaWindowCornerPreference, ref pref, sizeof(uint));
    }

    /// <summary>
    /// Remove the Win11 hairline border drawn around even frameless windows —
    /// it traces the square HWND bounds and breaks the notch silhouette.
    /// </summary>
    public static void DisableDwmBorder(nint hwnd)
    {
        if (hwnd == 0)
            return;
        var none = DwmwaColorNone;
        _ = DwmSetWindowAttribute(hwnd, DwmwaBorderColor, ref none, sizeof(uint));
    }

    /// <summary>
    /// Clip the HWND to a notch: flat top edge with CONCAVE top fillets
    /// ("wings" flaring into the screen edge, macOS-notch style), straight
    /// sides inset by the wing radius, and convex rounded bottom corners.
    /// This cuts everything square (backdrop, border remnants) regardless of
    /// backdrop/transparency behavior. Coordinates are physical px; call after
    /// every resize (the region is size-relative).
    /// </summary>
    public static void ApplyNotchRegion(
        nint hwnd, int width, int height, int bottomRadius, int wingRadius)
    {
        if (hwnd == 0 || width <= 0 || height <= 0)
            return;

        var rw = Math.Max(0, Math.Min(wingRadius, Math.Min(width / 4, height)));
        var slabLeft = rw;
        var slabRight = width - rw;
        var rb = Math.Max(0, Math.Min(bottomRadius, Math.Min((slabRight - slabLeft) / 2, height)));

        var rounded = CreateRoundRectRgn(
            slabLeft, 0, slabRight + 1, height + 1, rb * 2, rb * 2);
        var top = CreateRectRgn(slabLeft, 0, slabRight, Math.Max(1, height - rb));
        var region = CreateRectRgn(0, 0, 0, 0);
        _ = CombineRgn(region, rounded, top, RgnOr);
        _ = DeleteObject(rounded);
        _ = DeleteObject(top);

        if (rw > 0)
        {
            // Concave fillet = corner square minus a circle tangent to the
            // screen edge: full-width at y=0, tapering to the slab side at y=rw.
            AddWing(region, 0, rw, -rw, rw);
            AddWing(region, width - rw, width, width - rw, width + rw);
        }

        // On success the system owns the region; on failure we must free it.
        if (SetWindowRgn(hwnd, region, redraw: true) == 0)
            _ = DeleteObject(region);
    }

    private static void AddWing(nint region, int squareLeft, int squareRight, int circleLeft, int circleRight)
    {
        var rw = squareRight - squareLeft;
        var square = CreateRectRgn(squareLeft, 0, squareRight, rw);
        var circle = CreateEllipticRgn(circleLeft, 0, circleRight, rw * 2);
        var wing = CreateRectRgn(0, 0, 0, 0);
        _ = CombineRgn(wing, square, circle, RgnDiff);
        _ = CombineRgn(region, region, wing, RgnOr);
        _ = DeleteObject(square);
        _ = DeleteObject(circle);
        _ = DeleteObject(wing);
    }

    /// <summary>
    /// Strip the classic frame styles SetBorderAndTitleBar(false, false) leaves
    /// behind (WS_DLGFRAME / WS_THICKFRAME / WS_EX_WINDOWEDGE) — they paint a
    /// light 1px edge tracing the square HWND bounds through the notch shape.
    /// </summary>
    public static void StripClassicEdges(nint hwnd)
    {
        if (hwnd == 0)
            return;
        var style = (long)GetWindowLongPtrW(hwnd, GwlStyle);
        _ = SetWindowLongPtrW(hwnd, GwlStyle, (nint)(style & ~WsFrameBits));
        var ex = (long)GetWindowLongPtrW(hwnd, GwlExStyle);
        _ = SetWindowLongPtrW(hwnd, GwlExStyle, (nint)(ex & ~WsExEdgeBits));
        _ = SetWindowPos(hwnd, 0, 0, 0, 0, 0, SwpFrameChanged);
    }

    [DllImport("dwmapi.dll")]
    private static extern int DwmSetWindowAttribute(
        nint hwnd, uint attribute, ref uint value, int size);

    [DllImport("user32.dll", EntryPoint = "GetWindowLongPtrW")]
    private static extern nint GetWindowLongPtrW(nint hwnd, int index);

    [DllImport("user32.dll", EntryPoint = "SetWindowLongPtrW")]
    private static extern nint SetWindowLongPtrW(nint hwnd, int index, nint value);

    [DllImport("user32.dll")]
    private static extern bool SetWindowPos(
        nint hwnd, nint insertAfter, int x, int y, int width, int height, uint flags);

    /// <summary>
    /// Current cursor X in physical screen px. Drag math must use this rather
    /// than pointer-event coordinates: event positions are window-relative and
    /// sampled before the window moved, so pairing them with the live window
    /// position feeds the drag its own movement back (jitter/drift).
    /// </summary>
    public static bool TryGetCursorScreenX(out int x)
    {
        if (GetCursorPos(out var p))
        {
            x = p.X;
            return true;
        }
        x = 0;
        return false;
    }

    [DllImport("user32.dll")]
    private static extern bool GetCursorPos(out Point point);

    [StructLayout(LayoutKind.Sequential)]
    private struct Point
    {
        public int X;
        public int Y;
    }

    [DllImport("gdi32.dll")]
    private static extern nint CreateRoundRectRgn(
        int left, int top, int right, int bottom, int widthEllipse, int heightEllipse);

    [DllImport("gdi32.dll")]
    private static extern nint CreateRectRgn(int left, int top, int right, int bottom);

    [DllImport("gdi32.dll")]
    private static extern nint CreateEllipticRgn(int left, int top, int right, int bottom);

    [DllImport("gdi32.dll")]
    private static extern int CombineRgn(nint dest, nint src1, nint src2, int mode);

    [DllImport("gdi32.dll")]
    private static extern bool DeleteObject(nint obj);

    [DllImport("user32.dll")]
    private static extern int SetWindowRgn(nint hwnd, nint region, bool redraw);
}
