/**
 * Electron BrowserWindow factory — the ONLY production site that may call
 * `new BrowserWindow` (contract-migration-window-per-session-invariant lint).
 *
 * This module is intentionally free of a hard `electron` import so Linux CI
 * typecheck/vitest stay clean. Wire it from a Windows-only entry that passes
 * the Electron constructors in.
 */

import type { WindowBounds, WindowFactory, WindowHandle } from "./window-registry.js";
import type { DaemonConfigResolver, HostedModeInfo } from "./daemon-config.js";

/** Minimal surface of Electron BrowserWindow used by the factory. */
export interface ElectronBrowserWindowLike {
  focus(): void;
  close(): void;
  isDestroyed(): boolean;
  getBounds(): { x: number; y: number; width: number; height: number };
  setBounds(b: { x: number; y: number; width: number; height: number }): void;
  loadURL(url: string): Promise<void>;
  webContents: {
    once(event: "did-finish-load", listener: () => void): void;
    executeJavaScript(code: string): Promise<unknown>;
  };
}

export interface ElectronBrowserWindowConstructor {
  new (options: Record<string, unknown>): ElectronBrowserWindowLike;
}

export interface ElectronFactoryOptions {
  config: DaemonConfigResolver;
  BrowserWindow: ElectronBrowserWindowConstructor;
  preloadPath: string;
  loadUrl?: (sessionId: string, webOrigin: string) => string;
}

/**
 * Build the sole BrowserWindow factory. Token is injected after load into
 * `window.hostedModeInfo` for the SPA auth reader; production may replace the
 * injection with a contextBridge preload that receives the same payload via IPC.
 */
export function createElectronWindowFactory(opts: ElectronFactoryOptions): WindowFactory {
  const { BrowserWindow } = opts;

  return {
    create(sessionId: string, bounds?: WindowBounds): WindowHandle {
      const win = new BrowserWindow({
        x: bounds?.x,
        y: bounds?.y,
        width: bounds?.width ?? 1280,
        height: bounds?.height ?? 800,
        show: true,
        webPreferences: {
          contextIsolation: true,
          nodeIntegration: false,
          sandbox: true,
          preload: opts.preloadPath,
        },
      });

      void (async () => {
        try {
          const cfg = await opts.config.resolve();
          const url =
            opts.loadUrl?.(sessionId, cfg.webOrigin) ??
            opts.config.hostedUrl(cfg.webOrigin, sessionId);
          const info: HostedModeInfo = {
            hosted: true,
            sessionId,
            baseUrl: cfg.baseUrl,
            token: cfg.token,
          };
          win.webContents.once("did-finish-load", () => {
            void win.webContents.executeJavaScript(
              `window.hostedModeInfo = ${JSON.stringify(info)};`,
            );
          });
          await win.loadURL(url);
        } catch (e) {
          const msg = (e as Error).message;
          await win.loadURL(
            `data:text/html,${encodeURIComponent(`<h1>Connection error</h1><pre>${msg}</pre>`)}`,
          );
        }
      })();

      return {
        id: sessionId,
        focus: () => {
          if (!win.isDestroyed()) win.focus();
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
    },
  };
}
