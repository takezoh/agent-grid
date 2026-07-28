/**
 * Electron window factory — the ONLY production site that may construct
 * Electron windows/views (contract-single-fleet-window-construction
 * lint, scripts/check-browserwindow.mjs).
 *
 * Each Workspace window is a frameless BaseWindow split into two
 * WebContentsViews (see window-chrome.ts): a Workspace-owned title bar
 * (drag region + File/Edit/View/Window menu bar, VSCode-style) on top and
 * the unmodified shared SPA below. Native caption buttons stay via the
 * Window Controls Overlay.
 *
 * This module is intentionally free of a hard `electron` import so Linux CI
 * typecheck/vitest stay clean. Wire it from a Windows-only entry that passes
 * the Electron constructors in.
 */

import type { WindowBounds, WindowFactory, WindowHandle } from "./window-registry.js";
import type { DaemonConfigResolver, HostedModeInfo } from "./daemon-config.js";
import type { AppearanceConfig, WorkspaceAppConfig } from "./desktop-config.js";
import type { SessionRef } from "../shared/session-ref.js";
import { settingsPageUrl } from "./settings-page.js";
import {
  CHROME_COLORS,
  layoutChrome,
  resolveChromeTheme,
  titleBarDataUrl,
  titleBarOverlayFor,
  windowDisplayTitle,
  type ChromeRect,
} from "./window-chrome.js";

export type { ChromeRect } from "./window-chrome.js";

/** Minimal surfaces of Electron BaseWindow / WebContentsView used here. */
export interface ElectronWebContentsLike {
  readonly id: number;
  once(event: "did-finish-load", listener: () => void): void;
  executeJavaScript(code: string): Promise<unknown>;
  loadURL(url: string): Promise<void>;
  focus?(): void;
}

export interface ElectronWebContentsViewLike {
  webContents: ElectronWebContentsLike;
  setBounds(bounds: ChromeRect): void;
  setBackgroundColor?(color: string): void;
}

export interface ElectronWebContentsViewConstructor {
  new (options: Record<string, unknown>): ElectronWebContentsViewLike;
}

export interface ElectronBaseWindowLike {
  focus(): void;
  close(): void;
  isDestroyed(): boolean;
  getBounds(): ChromeRect;
  setBounds(bounds: ChromeRect): void;
  getContentBounds(): ChromeRect;
  on(event: "resize", listener: () => void): void;
  contentView: { addChildView(view: ElectronWebContentsViewLike): void };
}

export interface ElectronBaseWindowConstructor {
  new (options: Record<string, unknown>): ElectronBaseWindowLike;
}

export interface WindowChromeHandles {
  window: ElectronBaseWindowLike;
  titleBarWebContentsId: number;
  contentWebContentsId: number;
}

export interface ElectronFactoryOptions {
  config: DaemonConfigResolver;
  BaseWindow: ElectronBaseWindowConstructor;
  WebContentsView: ElectronWebContentsViewConstructor;
  /** Preload for the SPA content view. */
  preloadPath: string;
  /** Preload for the title-bar view (menu-popup bridge). */
  titleBarPreloadPath: string;
  /** file: URL of the UI bundled with Workspace. */
  uiEntryUrl: string;
  appearance?: AppearanceConfig;
  workspace?: WorkspaceAppConfig;
  getAppearance?: () => AppearanceConfig | undefined;
  getWorkspace?: () => WorkspaceAppConfig | undefined;
  /** OS dark-mode probe (nativeTheme in production; defaults to dark). */
  systemPrefersDark?: () => boolean;
  registerRendererInfo?: (webContentsId: number, info: HostedModeInfo) => void;
  configureServerTransport?: (webContentsId: number, baseUrl: string) => void;
  /** Lets the entry answer title-bar menu IPC / repaint chrome per window. */
  registerWindowChrome?: (handles: WindowChromeHandles) => void;
}

export interface ElectronWindowFactory extends WindowFactory {
  createHome(serverId: string): WindowHandle;
  createSettings(): WindowHandle;
}

export function sessionPageUrl(uiEntryUrl: string, sessionId?: string): string {
  const url = new URL(uiEntryUrl);
  if (url.protocol !== "file:") throw new Error("Workspace UI entry must be a file: URL");
  if (sessionId) {
    url.searchParams.set("hosted", "1");
    url.searchParams.set("session", sessionId);
  }
  return url.toString();
}

/**
 * Build the sole window factory. Token is injected after load into
 * `window.hostedModeInfo` for the SPA auth reader; production may replace the
 * injection with a contextBridge preload that receives the same payload via IPC.
 */
export function createElectronWindowFactory(opts: ElectronFactoryOptions): ElectronWindowFactory {
  const { BaseWindow, WebContentsView } = opts;

  const createWindow = (
    kind: "home" | "session" | "settings",
    serverId: string,
    sessionId?: string,
    bounds?: WindowBounds,
  ): WindowHandle => {
      const appearance = opts.getAppearance?.() ?? opts.appearance;
      const workspace = opts.getWorkspace?.() ?? opts.workspace;
      const chromeTheme = resolveChromeTheme(
        appearance?.theme,
        opts.systemPrefersDark?.() ?? true,
      );
      const overlay = titleBarOverlayFor(chromeTheme);
      const win = new BaseWindow({
        x: bounds?.x,
        y: bounds?.y,
        width: bounds?.width ?? workspace?.default_window.width ?? 1280,
        height: bounds?.height ?? workspace?.default_window.height ?? 800,
        show: true,
        titleBarStyle: "hidden",
        titleBarOverlay: overlay,
        // Pre-load background matching the theme so no white frame flashes.
        backgroundColor: overlay.color,
      });

      const titleBar = new WebContentsView({
        webPreferences: {
          contextIsolation: true,
          nodeIntegration: false,
          sandbox: true,
          preload: opts.titleBarPreloadPath,
        },
      });
      const content = new WebContentsView({
        webPreferences: {
          contextIsolation: true,
          nodeIntegration: false,
          sandbox: true,
          // The trusted bundled file: UI talks to the separately hosted API.
          // Keep it in a server-specific partition and remove WS Origin below.
          webSecurity: false,
          partition: `persist:agent-grid-${encodeURIComponent(serverId)}`,
          preload: opts.preloadPath,
        },
      });
      titleBar.setBackgroundColor?.(overlay.color);
      content.setBackgroundColor?.(overlay.color);
      win.contentView.addChildView(titleBar);
      win.contentView.addChildView(content);

      const applyLayout = () => {
        const layout = layoutChrome(win.getContentBounds());
        titleBar.setBounds(layout.titleBar);
        content.setBounds(layout.content);
      };
      applyLayout();
      win.on("resize", applyLayout);

      void titleBar.webContents.loadURL(
        titleBarDataUrl(
          chromeTheme,
          kind === "settings" ? "Settings" : windowDisplayTitle(serverId, sessionId),
        ),
      );
      opts.registerWindowChrome?.({
        window: win,
        titleBarWebContentsId: titleBar.webContents.id,
        contentWebContentsId: content.webContents.id,
      });

      if (kind === "settings") {
        void content.webContents.loadURL(settingsPageUrl());
        return {
          id: "settings",
          focus: () => {
            if (win.isDestroyed()) return;
            win.focus();
            content.webContents.focus?.();
          },
          close: () => {
            if (!win.isDestroyed()) win.close();
          },
          isDestroyed: () => win.isDestroyed(),
          getBounds: () => {
            if (win.isDestroyed()) return { x: 0, y: 0, width: 0, height: 0 };
            const b = win.getBounds();
            return { x: b.x, y: b.y, width: b.width, height: b.height };
          },
          setBounds: (b) => {
            if (!win.isDestroyed()) win.setBounds(b);
          },
        };
      }

      if (kind === "home") {
        void (async () => {
          try {
            const cfg = await opts.config.resolve(serverId);
            const url = sessionPageUrl(opts.uiEntryUrl);
            const info: HostedModeInfo = {
              hosted: false,
              serverId: cfg.serverId,
              serverLabel: cfg.serverLabel,
              baseUrl: cfg.baseUrl,
              token: cfg.token,
            };
            opts.registerRendererInfo?.(content.webContents.id, info);
            opts.configureServerTransport?.(content.webContents.id, cfg.baseUrl);
            content.webContents.once("did-finish-load", () => {
              void content.webContents.executeJavaScript(
                `window.hostedModeInfo = ${JSON.stringify(info)};` +
                `window.agentGridAppearance = ${JSON.stringify(appearance ?? null)};` +
                `window.dispatchEvent(new CustomEvent("agent-grid-appearance"));`,
              );
            });
            await content.webContents.loadURL(url);
          } catch (e) {
            const msg = (e as Error).message;
            const colors = CHROME_COLORS[chromeTheme];
            const html =
              `<body style="margin:0;background:${colors.bg};color:${colors.fg};font-family:system-ui">` +
              `<main style="padding:16px 24px"><h1>Connection error</h1><pre>${msg}</pre></main></body>`;
            await content.webContents.loadURL(`data:text/html,${encodeURIComponent(html)}`);
          }
        })();
        return {
          id: `home:${serverId}`,
          focus: () => {
            if (win.isDestroyed()) return;
            win.focus();
            content.webContents.focus?.();
          },
          close: () => {
            if (!win.isDestroyed()) win.close();
          },
          isDestroyed: () => win.isDestroyed(),
          getBounds: () => {
            if (win.isDestroyed()) return { x: 0, y: 0, width: 0, height: 0 };
            const b = win.getBounds();
            return { x: b.x, y: b.y, width: b.width, height: b.height };
          },
          setBounds: (b) => {
            if (!win.isDestroyed()) win.setBounds(b);
          },
        };
      }

      void (async () => {
        try {
          const cfg = await opts.config.resolve(serverId);
          const url = sessionPageUrl(opts.uiEntryUrl, sessionId);
          const info: HostedModeInfo = {
            hosted: sessionId !== undefined,
            ...(sessionId === undefined ? {} : { sessionId }),
            serverId: cfg.serverId,
            serverLabel: cfg.serverLabel,
            baseUrl: cfg.baseUrl,
            token: cfg.token,
          };
          opts.registerRendererInfo?.(content.webContents.id, info);
          opts.configureServerTransport?.(content.webContents.id, cfg.baseUrl);
          content.webContents.once("did-finish-load", () => {
            void content.webContents.executeJavaScript(
              `window.hostedModeInfo = ${JSON.stringify(info)};` +
              `window.agentGridAppearance = ${JSON.stringify(appearance ?? null)};` +
              `window.dispatchEvent(new CustomEvent("agent-grid-appearance"));`,
            );
          });
          await content.webContents.loadURL(url);
        } catch (e) {
          const msg = (e as Error).message;
          const colors = CHROME_COLORS[chromeTheme];
          const html =
            `<body style="margin:0;background:${colors.bg};color:${colors.fg};font-family:system-ui">` +
            `<main style="padding:16px 24px"><h1>Connection error</h1><pre>${msg}</pre></main></body>`;
          await content.webContents.loadURL(`data:text/html,${encodeURIComponent(html)}`);
        }
      })();

      return {
        id: `${serverId}:${sessionId}`,
        focus: () => {
          if (win.isDestroyed()) return;
          win.focus();
          content.webContents.focus?.();
        },
        close: () => {
          if (!win.isDestroyed()) win.close();
        },
        isDestroyed: () => win.isDestroyed(),
        getBounds: () => {
          if (win.isDestroyed()) return { x: 0, y: 0, width: 0, height: 0 };
          const b = win.getBounds();
          return { x: b.x, y: b.y, width: b.width, height: b.height };
        },
        setBounds: (b) => {
          if (!win.isDestroyed()) win.setBounds(b);
        },
        selectSession: (session) => {
          if (win.isDestroyed()) return;
          void content.webContents.executeJavaScript(
            `window.dispatchEvent(new CustomEvent("agent-grid-select-session",{detail:${JSON.stringify(
              session,
            )}}));`,
          );
        },
      };
  };

  return {
    create(session: SessionRef, bounds?: WindowBounds): WindowHandle {
      return createWindow("session", session.serverId, session.sessionId, bounds);
    },
    createHome(serverId: string): WindowHandle {
      return createWindow("home", serverId);
    },
    createSettings(): WindowHandle {
      return createWindow("settings", "__settings__");
    },
  };
}
