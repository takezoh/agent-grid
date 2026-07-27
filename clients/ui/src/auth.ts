/**
 * Bearer token resolution for browser and hosted (Electron Workspace) modes.
 *
 * Browser mode: `#token=...` hash fragment (existing contract).
 * Hosted mode: `window.hostedModeInfo.token` injected by Workspace preload
 * (contract-b2-hosted-mode-token-injection). Token MUST NOT appear in the URL.
 */

export interface HostedModeInfo {
  hosted: boolean;
  sessionId?: string;
  serverId?: string;
  serverLabel?: string;
  baseUrl: string;
  token: string;
}

export interface FleetHealth {
  checkedAt: string;
  servers: Array<{
    serverId: string;
    displayName: string;
    state: "connected" | "unavailable" | "disabled";
    reason?: string;
  }>;
}

export interface DesktopAppearance {
  theme: "default" | "system" | "light" | "dark";
  density: "compact" | "comfortable";
  font_scale: number;
}

declare global {
  interface Window {
    hostedModeInfo?: HostedModeInfo;
    agentGridAppearance?: DesktopAppearance | null;
    agentGridWorkspace?: {
      hostedModeInfo?: HostedModeInfo;
      /**
       * Hosted chrome: re-syncs the native chrome (caption buttons, title
       * bar, popup menus). `mode` is the user's choice — "system" keeps
       * native UI following the OS palette.
       */
      setTitleBarTheme?: (theme: "dark" | "light", mode?: "system" | "light" | "dark") => void;
      serverHealth?: { list(): Promise<FleetHealth> };
      sessionFleet?: {
        listConfigs(): Promise<unknown>;
        create(serverId: string, payload: unknown): Promise<unknown>;
      };
    };
  }
}

export function desktopAppearance(): DesktopAppearance | null {
  if (typeof window === "undefined") return null;
  return window.agentGridAppearance ?? null;
}

export function isHostedMode(): boolean {
  if (typeof window === "undefined") return false;
  if (window.hostedModeInfo?.hosted) return true;
  if (window.agentGridWorkspace?.hostedModeInfo?.hosted) return true;
  try {
    const params = new URLSearchParams(window.location.search);
    return params.get("hosted") === "1";
  } catch {
    return false;
  }
}

export function hostedSessionId(): string | null {
  const fromBridge =
    window.hostedModeInfo?.sessionId ?? window.agentGridWorkspace?.hostedModeInfo?.sessionId;
  if (fromBridge) return fromBridge;
  try {
    return new URLSearchParams(window.location.search).get("session");
  } catch {
    return null;
  }
}

export function desktopBaseUrl(): string | null {
  const value =
    window.hostedModeInfo?.baseUrl ?? window.agentGridWorkspace?.hostedModeInfo?.baseUrl;
  return value ? value.replace(/\/+$/, "") : null;
}

export function currentServerIdentity(): { id: string; label: string } {
  const hosted = window.hostedModeInfo ?? window.agentGridWorkspace?.hostedModeInfo;
  if (hosted?.serverId) {
    return { id: hosted.serverId, label: hosted.serverLabel || hosted.serverId };
  }
  const serverId = new URLSearchParams(window.location.search).get("server");
  if (!serverId) throw new Error("No server selected");
  return { id: serverId, label: serverId };
}

export function resolveApiUrl(pathname: string): string {
  const baseUrl = desktopBaseUrl();
  return baseUrl ? new URL(pathname, `${baseUrl}/`).toString() : pathname;
}

export function readBearerTokenFromHash(): string {
  // Hosted mode: preload-injected token takes precedence (never from URL).
  const hosted = window.hostedModeInfo?.token ?? window.agentGridWorkspace?.hostedModeInfo?.token;
  if (hosted) return hosted;

  const hash = window.location.hash; // e.g. "#token=abc"
  if (!hash.startsWith("#")) return "";
  const params = new URLSearchParams(hash.slice(1));
  return params.get("token") ?? "";
}
