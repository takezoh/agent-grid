/**
 * Boundary-2 adapter on the Workspace side: resolves port/token for hosted SPA.
 * Token is never placed in URL (contract-b2-hosted-mode-token-injection, FR-B2-04).
 * Fresh file read each resolve (contract-b2-token-acquisition).
 */

import * as fs from "node:fs/promises";

export interface DaemonConfig {
  baseUrl: string;
  webOrigin: string;
  token: string;
  tokenPath: string;
}

export interface DaemonConfigSource {
  /** Absolute path to the gateway bearer token file (UNC on Windows). */
  tokenPath: string;
  /** Gateway base URL, e.g. http://127.0.0.1:8443 */
  baseUrl: string;
  /** SPA origin served by uihost, e.g. http://127.0.0.1:5173 */
  webOrigin: string;
}

export class DaemonConfigResolver {
  constructor(private readonly source: DaemonConfigSource) {}

  /**
   * Fresh-read token every call. Throws if unreadable — callers surface
   * connection-error view, never fabricate Connected (FR-B2-03).
   */
  async resolve(): Promise<DaemonConfig> {
    let token: string;
    try {
      token = (await fs.readFile(this.source.tokenPath, "utf8")).trim();
    } catch (e) {
      throw new Error(
        `gateway token unreadable at '${this.source.tokenPath}': ${(e as Error).message}`,
      );
    }
    if (!token) {
      throw new Error(`gateway token empty at '${this.source.tokenPath}'`);
    }
    return {
      baseUrl: this.source.baseUrl,
      webOrigin: this.source.webOrigin,
      token,
      tokenPath: this.source.tokenPath,
    };
  }

  /**
   * Build the hosted-mode URL. Token is NEVER in the query string.
   * Token is injected via preload contextBridge (window.hostedModeInfo).
   */
  hostedUrl(webOrigin: string, sessionId: string): string {
    const u = new URL(webOrigin);
    u.searchParams.set("hosted", "1");
    u.searchParams.set("session", sessionId);
    // Deliberately no token= param.
    return u.toString();
  }
}

/** Info pushed into the renderer via contextBridge. */
export interface HostedModeInfo {
  hosted: true;
  sessionId: string;
  baseUrl: string;
  /** Bearer token — only via preload, never URL. */
  token: string;
}
