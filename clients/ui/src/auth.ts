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
  hostInstanceId?: string;
  revision?: number;
  servers: Array<{
    serverId: string;
    displayName: string;
    state:
      | "connected"
      | "unavailable"
      | "incompatible_protocol"
      | "disabled"
      | "checking"
      | "health_unavailable";
    reason?: string;
    stale?: boolean;
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
      openSettings?: (section: "github" | "connections") => Promise<void>;
      /** Credential-free GitHub Settings actions (Workspace main/preload). */
      github?: {
        action(
          action: import("./store/repository_picker").GitHubAction,
        ): Promise<import("./store/repository_picker").GitHubActionResult>;
        openAuthorization(): Promise<void>;
      };
      sessionFleet?: {
        listConfigs(): Promise<unknown>;
        subscribe?(listener: (snapshot: unknown) => void): () => void;
        subscribeHealth?(listener: (snapshot: unknown) => void): () => void;
        configPath?(): Promise<string | null>;
        openChannel?(
          serverId: string,
          listener: (event: { channelId: string; kind: string; data?: unknown }) => void,
        ): Promise<{ channelId: string; send(data: string): void; close(): void }>;
        create(serverId: string, payload: unknown): Promise<unknown>;
        repositoryProjection?(): Promise<
          import("./store/repository_picker").HostRepositoryProjection
        >;
        createRepositorySession?(serverId: string, payload: unknown): Promise<unknown>;
        /** Host-mediated per-server reconnect (Workspace FleetSessionRuntime). */
        reconnect?(serverId: string): Promise<void>;
        request?(
          serverId: string,
          pathname: string,
          init: { method?: string; headers?: Record<string, string>; body?: string },
        ): Promise<{
          status: number;
          headers: Record<string, string>;
          /**
           * `null` means "no body", which is the only thing a null-body status
           * (101/204/205/304) may carry — `""` is an empty body and is a
           * different fact. Main owns that invariant; this declaration is the
           * consumer half and is widened first (see the rollout note in
           * fleet/provider.ts).
           */
          body: string | null;
        }>;
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

export function initialServerHint(): { id: string; label: string } {
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
