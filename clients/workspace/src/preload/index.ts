/**
 * Typed minimal contextBridge surface (boundary 4).
 * contextIsolation: true, nodeIntegration: false, sandbox: true.
 * Never exposes Node APIs to the renderer.
 *
 * Hosted-mode token injection lives here (contract-b2-hosted-mode-token-injection).
 */

import type { DesktopConfig, EditableDesktopSettings } from "../main/desktop-config.js";
import type { FleetHealth } from "../main/server-health.js";
import type { GitHubAction, GitHubActionResult } from "../main/repository-lease.js";

export interface HostedModeInfo {
  hosted: boolean;
  sessionId?: string;
  serverId?: string;
  serverLabel?: string;
  baseUrl: string;
  token: string;
}

export interface DesktopConfigBridge {
  load(): Promise<DesktopConfig>;
  save(settings: EditableDesktopSettings): Promise<DesktopConfig>;
}

export interface WindowControls {
  minimize(): void;
  maximize(): void;
  close(): void;
}

export interface GitHubSettingsBridge {
  action(action: GitHubAction): Promise<GitHubActionResult>;
  openAuthorization(): Promise<void>;
}

export interface WorkspacePreloadApi {
  windowControls: WindowControls;
  hostedModeInfo: HostedModeInfo | null;
  requestJumpBack(sessionId: string): Promise<void>;
  desktopConfig?: DesktopConfigBridge;
  serverHealth?: { list(): Promise<FleetHealth> };
  openSettings?: (section: "github") => Promise<void>;
  github?: GitHubSettingsBridge;
  sessionFleet?: {
    listConfigs(): Promise<unknown>;
    subscribe(listener: (snapshot: unknown) => void): () => void;
    subscribeHealth?(listener: (snapshot: unknown) => void): () => void;
    configPath?(): Promise<string | null>;
    openChannel(
      serverId: string,
      listener: (event: { channelId: string; kind: string; data?: unknown }) => void,
    ): Promise<{ channelId: string; send(data: string): void; close(): void }>;
    create(serverId: string, payload: unknown): Promise<unknown>;
    createRepositorySession(serverId: string, payload: unknown): Promise<unknown>;
    request(
      serverId: string,
      pathname: string,
      init: { method?: string; headers?: Record<string, string>; body?: string },
    ): Promise<{ status: number; headers: Record<string, string>; body: string }>;
  };
  /**
   * Re-syncs the native chrome (caption-button overlay, title-bar view,
   * popup menus) with the SPA theme. `mode` is the user's choice —
   * "system" keeps native UI following the OS palette.
   */
  setTitleBarTheme?(theme: "dark" | "light", mode?: "system" | "light" | "dark"): void;
}

/**
 * Pure builder used by tests and by the Electron preload entry.
 * The actual `contextBridge.exposeInMainWorld` call is Electron-only.
 */
export function buildPreloadApi(opts: {
  hostedModeInfo: HostedModeInfo | null;
  windowControls: WindowControls;
  requestJumpBack: (sessionId: string) => Promise<void>;
  desktopConfig?: DesktopConfigBridge;
  serverHealth?: { list(): Promise<FleetHealth> };
  openSettings?: (section: "github") => Promise<void>;
  github?: GitHubSettingsBridge;
  sessionFleet?: WorkspacePreloadApi["sessionFleet"];
}): WorkspacePreloadApi {
  return {
    windowControls: opts.windowControls,
    hostedModeInfo: opts.hostedModeInfo,
    requestJumpBack: opts.requestJumpBack,
    desktopConfig: opts.desktopConfig,
    serverHealth: opts.serverHealth,
    openSettings: opts.openSettings,
    github: opts.github,
    sessionFleet: opts.sessionFleet,
  };
}

/**
 * Assert that a navigation URL never carries the token (T1 criterion).
 */
export function assertTokenNotInUrl(url: string, token: string): void {
  if (!token) return;
  if (url.includes(token)) {
    throw new Error("token must not appear in hosted-mode URL");
  }
  try {
    const u = new URL(url);
    if (u.searchParams.get("token")) {
      throw new Error("token query param forbidden in hosted-mode URL");
    }
    if (u.hash.includes("token=")) {
      throw new Error("token hash fragment forbidden in hosted-mode URL");
    }
  } catch (e) {
    if (e instanceof TypeError) return; // relative URL — still check raw includes above
    throw e;
  }
}
