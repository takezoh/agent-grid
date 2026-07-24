/**
 * Boundary-2 adapter on the Workspace side: resolves port/token for hosted SPA.
 * Token is never placed in URL (contract-b2-hosted-mode-token-injection, FR-B2-04).
 * Fresh file read each resolve (contract-b2-token-acquisition).
 */

import * as fs from "node:fs/promises";
import type { ServerConfig } from "./desktop-config.js";

export interface DaemonConfig {
  baseUrl: string;
  token: string;
  tokenPath: string;
}

export interface DaemonConfigSource {
  /** Stable client-local server identifier. */
  serverId: string;
  /** Absolute path to the gateway bearer token file (UNC on Windows). */
  tokenPath: string;
  /** Gateway base URL, e.g. http://127.0.0.1:8443 */
  baseUrl: string;
}

export class DaemonConfigResolver {
  private readonly sources: ReadonlyMap<string, DaemonConfigSource>;

  constructor(sources: DaemonConfigSource | readonly DaemonConfigSource[]) {
    const values = Array.isArray(sources) ? sources : [sources];
    this.sources = new Map(values.map((source) => [source.serverId, source]));
  }

  static fromServers(servers: readonly ServerConfig[]): DaemonConfigResolver {
    return new DaemonConfigResolver(
      servers.filter((server) => server.enabled).map((server) => ({
        serverId: server.id,
        tokenPath: server.token_path,
        baseUrl: server.url,
      })),
    );
  }

  /**
   * Fresh-read token every call. Throws if unreadable — callers surface
   * connection-error view, never fabricate Connected (FR-B2-03).
   */
  async resolve(serverId: string): Promise<DaemonConfig> {
    const source = this.sources.get(serverId);
    if (!source) throw new Error(`unknown or disabled server '${serverId}'`);
    let token: string;
    try {
      token = (await fs.readFile(source.tokenPath, "utf8")).trim();
    } catch (e) {
      throw new Error(
        `gateway token unreadable at '${source.tokenPath}': ${(e as Error).message}`,
      );
    }
    if (!token) {
      throw new Error(`gateway token empty at '${source.tokenPath}'`);
    }
    return {
      baseUrl: source.baseUrl,
      token,
      tokenPath: source.tokenPath,
    };
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
