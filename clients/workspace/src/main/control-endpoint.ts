/**
 * Boundary-1 adapter on the Workspace side: JSON Lines control server.
 * On Windows: named pipe \\.\pipe\agent-grid-workspace
 * On Linux/test: Unix domain socket path (injectable for vitest).
 *
 * No domain data on this pipe — only openSession / activate / quit.
 */

import * as net from "node:net";
import { parseControlLine, replyError, replyOk } from "../shared/control-envelope.js";
import type { WindowRegistry } from "./window-registry.js";

export const DEFAULT_PIPE_NAME = "agent-grid-workspace";

export function defaultControlPath(): string {
  if (process.platform === "win32") {
    return `\\\\.\\pipe\\${DEFAULT_PIPE_NAME}`;
  }
  // Non-Windows: abstract or filesystem socket for local dev/tests.
  return process.env.AG_WORKSPACE_CONTROL_PATH ?? `/tmp/${DEFAULT_PIPE_NAME}.sock`;
}

export interface ControlEndpointOptions {
  path?: string;
  registry: WindowRegistry;
  onQuit?: () => void;
}

export class ControlEndpoint {
  private server: net.Server | null = null;
  private readonly path: string;
  private readonly registry: WindowRegistry;
  private readonly onQuit: (() => void) | undefined;

  constructor(opts: ControlEndpointOptions) {
    this.path = opts.path ?? defaultControlPath();
    this.registry = opts.registry;
    this.onQuit = opts.onQuit;
  }

  get listenPath(): string {
    return this.path;
  }

  async start(): Promise<void> {
    if (this.server) return;
    // Clean stale Unix socket.
    if (process.platform !== "win32") {
      const fs = await import("node:fs/promises");
      try {
        await fs.unlink(this.path);
      } catch {
        /* absent is fine */
      }
    }
    await new Promise<void>((resolve, reject) => {
      const server = net.createServer((socket) => this.handleConnection(socket));
      server.once("error", reject);
      server.listen(this.path, () => {
        server.off("error", reject);
        this.server = server;
        resolve();
      });
    });
  }

  async stop(): Promise<void> {
    const server = this.server;
    this.server = null;
    if (!server) return;
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }

  private handleConnection(socket: net.Socket): void {
    let buf = "";
    socket.setEncoding("utf8");
    socket.on("data", (chunk: string) => {
      buf += chunk;
      let idx: number;
      while ((idx = buf.indexOf("\n")) >= 0) {
        const line = buf.slice(0, idx);
        buf = buf.slice(idx + 1);
        void this.dispatchLine(socket, line);
      }
    });
  }

  private async dispatchLine(socket: net.Socket, line: string): Promise<void> {
    const parsed = parseControlLine(line);
    if (!parsed.ok) {
      socket.write(`${replyError(parsed.error)}\n`);
      return;
    }
    try {
      switch (parsed.envelope.op) {
        case "openSession": {
          await this.registry.openSession({
            serverId: parsed.envelope.server_id!,
            sessionId: parsed.envelope.session_id!,
          });
          socket.write(`${replyOk()}\n`);
          break;
        }
        case "activate": {
          // Focus any open window; no-op if none.
          const sessions = this.registry.listSessions();
          if (sessions[0]) {
            await this.registry.openSession(sessions[0]);
          }
          socket.write(`${replyOk()}\n`);
          break;
        }
        case "quit": {
          socket.write(`${replyOk()}\n`);
          this.onQuit?.();
          break;
        }
      }
    } catch (e) {
      socket.write(`${replyError((e as Error).message)}\n`);
    }
  }
}
