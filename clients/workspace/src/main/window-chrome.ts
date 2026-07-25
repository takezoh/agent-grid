/**
 * Window chrome — VSCode-style custom title bar, owned by Workspace.
 *
 * The OS frame is replaced by a frameless window (titleBarStyle:"hidden" +
 * Window Controls Overlay: native caption buttons with Win11 Snap Layouts).
 * Instead of teaching the shared SPA to double as a title bar, the window is
 * split into two WebContentsViews:
 *
 *   ┌──────────────────────────────────────────────┐
 *   │ title-bar view (this module's HTML)  [─ □ ×] │  ← drag region + menus
 *   ├──────────────────────────────────────────────┤
 *   │ content view (unmodified shared SPA)         │
 *   └──────────────────────────────────────────────┘
 *
 * This module is pure (no electron import): colors, layout math, the menu-bar
 * contract, and the title-bar document. The factory wires it to real views;
 * electron-entry answers the title-bar IPC with native popup menus.
 */

export type ChromeTheme = "dark" | "light";

export interface TitleBarOverlayColors {
  color: string;
  symbolColor: string;
  height: number;
}

/** Matches --header-height in clients/ui tokens.css. */
export const TITLE_BAR_HEIGHT = 38;

/** Mirrors the SPA theme tokens (tokens.css --bg / --fg resolved values). */
export const CHROME_COLORS: Record<ChromeTheme, { bg: string; fg: string }> = {
  dark: { bg: "#0b0c0e", fg: "#e8eaee" },
  light: { bg: "#fafafa", fg: "#17181c" },
};

export function titleBarOverlayFor(theme: ChromeTheme): TitleBarOverlayColors {
  const colors = CHROME_COLORS[theme];
  return { color: colors.bg, symbolColor: colors.fg, height: TITLE_BAR_HEIGHT };
}

/**
 * Initial chrome theme at window creation. "default" defers to the SPA's own
 * resolution (stored choice / OS), which we cannot read from the main process
 * — we seed from the OS and the renderer corrects the chrome on first paint
 * through the setTitleBarTheme bridge.
 */
export function resolveChromeTheme(
  theme: "default" | "system" | "light" | "dark" | undefined,
  prefersDark: boolean,
): ChromeTheme {
  if (theme === "light" || theme === "dark") return theme;
  return prefersDark ? "dark" : "light";
}

export type ThemeSource = "system" | "light" | "dark";

/**
 * nativeTheme.themeSource for the SPA-reported theme mode: native popup menus
 * (the title-bar menu bar, context menus) follow nativeTheme, so mirroring
 * the SPA keeps them in the app palette, VSCode-style. Explicit choices pin
 * the palette; "system" (or an unknown payload) must stay "system" — pinning
 * it would freeze prefers-color-scheme in the renderers and the SPA's
 * system-mode resolution could never see the real OS theme again.
 */
export function themeSourceFor(mode: unknown): ThemeSource {
  return mode === "light" || mode === "dark" ? mode : "system";
}

// ─── Menu bar contract ───────────────────────────────────────────────────────

/**
 * Top-level menus rendered by the title bar. Clicking one sends
 * `agent-grid:titlebar-menu` {id,x,y}; electron-entry answers with a native
 * popup menu built for that id (submenu contents live there — they need the
 * live window / webContents).
 */
export const TITLE_BAR_MENUS = [
  { id: "file", label: "File" },
  { id: "edit", label: "Edit" },
  { id: "view", label: "View" },
  { id: "window", label: "Window" },
] as const;

export type TitleBarMenuId = (typeof TITLE_BAR_MENUS)[number]["id"];

export function isTitleBarMenuId(value: unknown): value is TitleBarMenuId {
  return TITLE_BAR_MENUS.some((menu) => menu.id === value);
}

// ─── Layout ──────────────────────────────────────────────────────────────────

export interface ChromeRect {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface ChromeLayout {
  titleBar: ChromeRect;
  content: ChromeRect;
}

/** Splits the window's content area into the title-bar strip and the SPA. */
export function layoutChrome(bounds: { width: number; height: number }): ChromeLayout {
  const width = Math.max(0, bounds.width);
  const height = Math.max(0, bounds.height);
  const bar = Math.min(TITLE_BAR_HEIGHT, height);
  return {
    titleBar: { x: 0, y: 0, width, height: bar },
    content: { x: 0, y: bar, width, height: height - bar },
  };
}

// ─── Window title ────────────────────────────────────────────────────────────

export function windowDisplayTitle(serverId: string, sessionId?: string): string {
  return sessionId ? `${serverId} · ${sessionId}` : "Agent Grid";
}

// ─── Title-bar document ──────────────────────────────────────────────────────

function escapeHtml(text: string): string {
  return text
    .replace(/&/g, "&amp;")
    .replace(/</g, "&lt;")
    .replace(/>/g, "&gt;")
    .replace(/"/g, "&quot;");
}

/**
 * Width reserved for the native caption buttons (min/max/close) that the
 * Window Controls Overlay paints over the title bar's right edge. env(
 * titlebar-area-*) is not exposed to child WebContentsViews, so the strip is
 * a fixed Windows-metrics reservation.
 */
export const CAPTION_BUTTONS_RESERVE = 146;

/**
 * The complete title-bar page. Self-contained (inline CSS/JS, both theme
 * palettes); the initial data-theme seeds from the chrome theme and
 * electron-entry re-stamps it when the SPA reports a theme switch.
 */
export function buildTitleBarHtml(theme: ChromeTheme, title: string): string {
  const menuButtons = TITLE_BAR_MENUS.map(
    (menu) => `<button data-menu="${menu.id}">${menu.label}</button>`,
  ).join("");
  return (
    `<!doctype html><html data-theme="${theme}"><head><meta charset="utf-8"><style>` +
    "html,body{margin:0;height:100%;overflow:hidden;user-select:none}" +
    "body{-webkit-app-region:drag;app-region:drag;display:flex;align-items:center;" +
    'gap:2px;padding:0 8px 0 10px;box-sizing:border-box;position:relative;' +
    'font:13px "Segoe UI",system-ui,sans-serif}' +
    `html[data-theme="dark"] body{background:${CHROME_COLORS.dark.bg};color:${CHROME_COLORS.dark.fg}}` +
    `html[data-theme="light"] body{background:${CHROME_COLORS.light.bg};color:${CHROME_COLORS.light.fg}}` +
    ".logo{width:14px;height:14px;border-radius:4px;margin-right:6px;flex:none;" +
    "background:linear-gradient(135deg,#6b74e0,#4650d8)}" +
    "button{-webkit-app-region:no-drag;app-region:no-drag;appearance:none;border:0;" +
    "background:transparent;color:inherit;font:inherit;padding:3px 9px;border-radius:5px}" +
    'html[data-theme="dark"] button:hover{background:rgba(232,234,238,.09)}' +
    'html[data-theme="light"] button:hover{background:rgba(23,24,28,.08)}' +
    ".title{position:absolute;left:50%;top:0;height:100%;transform:translateX(-50%);" +
    "display:flex;align-items:center;max-width:38vw;overflow:hidden;white-space:nowrap;" +
    "text-overflow:ellipsis;font-size:12px;opacity:.72;pointer-events:none}" +
    `.caption-pad{width:${CAPTION_BUTTONS_RESERVE}px;flex:none;margin-left:auto}` +
    "</style></head><body>" +
    '<div class="logo"></div>' +
    menuButtons +
    `<div class="title">${escapeHtml(title)}</div>` +
    '<div class="caption-pad"></div>' +
    "<script>" +
    'for (const b of document.querySelectorAll("button[data-menu]")) {' +
    "b.addEventListener('click', () => {" +
    "const r = b.getBoundingClientRect();" +
    "window.agentGridTitleBar?.openMenu(b.dataset.menu, r.left, r.bottom);" +
    "});" +
    "}" +
    "</script></body></html>"
  );
}

export function titleBarDataUrl(theme: ChromeTheme, title: string): string {
  return `data:text/html;charset=utf-8,${encodeURIComponent(buildTitleBarHtml(theme, title))}`;
}
