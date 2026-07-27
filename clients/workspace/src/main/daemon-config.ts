/**
 * Boundary-2 adapter on the Workspace side: resolves URL/auth for hosted SPA.
 * Token is never placed in URL (contract-b2-hosted-mode-token-injection, FR-B2-04).
 */
import type { ServerAuth, ServerConfig } from "./desktop-config.js";

export interface DaemonConfig {
  baseUrl: string;
  token: string;
}

export interface DaemonConfigSource {
  /** Stable identifier derived from the canonical gateway URL by every client. */
  serverId: string;
  auth: ServerAuth;
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
        auth: server.auth,
        baseUrl: server.url,
      })),
    );
  }

  /**
   * Resolves the validated persisted auth value without filesystem access.
   */
  async resolve(serverId: string): Promise<DaemonConfig> {
    const source = this.sources.get(serverId);
    if (!source) throw new Error(`unknown or disabled server '${serverId}'`);
    return {
      baseUrl: source.baseUrl,
      token: source.auth.mode === "bearer" ? source.auth.token : "",
    };
  }

}

/** Info pushed into the renderer via contextBridge. */
export interface HostedModeInfo {
  hosted: boolean;
  sessionId?: string;
  baseUrl: string;
  /** Bearer token — only via preload, never URL. */
  token: string;
}
