/**
 * Bearer token resolution for browser and hosted (Electron Workspace) modes.
 *
 * Browser mode: `#token=...` hash fragment (existing contract).
 * Hosted mode: `window.hostedModeInfo.token` injected by Workspace preload
 * (contract-b2-hosted-mode-token-injection). Token MUST NOT appear in the URL.
 */

export interface HostedModeInfo {
  hosted: true;
  sessionId: string;
  baseUrl: string;
  token: string;
}

declare global {
  interface Window {
    hostedModeInfo?: HostedModeInfo;
    agentGridWorkspace?: {
      hostedModeInfo?: HostedModeInfo;
    };
  }
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

export function readBearerTokenFromHash(): string {
  // Hosted mode: preload-injected token takes precedence (never from URL).
  const hosted =
    window.hostedModeInfo?.token ?? window.agentGridWorkspace?.hostedModeInfo?.token;
  if (hosted) return hosted;

  const hash = window.location.hash; // e.g. "#token=abc"
  if (!hash.startsWith("#")) return "";
  const params = new URLSearchParams(hash.slice(1));
  return params.get("token") ?? "";
}
