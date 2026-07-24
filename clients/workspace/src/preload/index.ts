/**
 * Typed minimal contextBridge surface (boundary 4).
 * contextIsolation: true, nodeIntegration: false, sandbox: true.
 * Never exposes Node APIs to the renderer.
 *
 * Hosted-mode token injection lives here (contract-b2-hosted-mode-token-injection).
 */

export interface HostedModeInfo {
  hosted: true;
  sessionId: string;
  baseUrl: string;
  token: string;
}

export interface WindowControls {
  minimize(): void;
  maximize(): void;
  close(): void;
}

export interface WorkspacePreloadApi {
  windowControls: WindowControls;
  hostedModeInfo: HostedModeInfo | null;
  requestJumpBack(sessionId: string): Promise<void>;
}

/**
 * Pure builder used by tests and by the Electron preload entry.
 * The actual `contextBridge.exposeInMainWorld` call is Electron-only.
 */
export function buildPreloadApi(opts: {
  hostedModeInfo: HostedModeInfo | null;
  windowControls: WindowControls;
  requestJumpBack: (sessionId: string) => Promise<void>;
}): WorkspacePreloadApi {
  return {
    windowControls: opts.windowControls,
    hostedModeInfo: opts.hostedModeInfo,
    requestJumpBack: opts.requestJumpBack,
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
