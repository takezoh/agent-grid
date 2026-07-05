import type { Page } from "@playwright/test";
import type { CreateSessionPayload } from "../../src/api/sessions";
import type { ServerFrame, SessionInfo, ViewUpdateFrame } from "../../src/wire/server";

export type FakeBackendOptions = {
  sessions: SessionInfo[];
  sessionConfig: {
    project_roots: string[];
    project_paths: string[];
    projects: Array<{ path: string; isGit: boolean; isSandboxed: boolean }>;
    commands: string[];
    push_commands: string[];
  };
  ticket?: string;
  createdSessionId?: string;
};

export type FakeBackend = {
  waitForSocketOpen(): Promise<void>;
  emit(frame: ServerFrame): Promise<void>;
  emittedFrames(): Promise<Array<Record<string, unknown>>>;
  sentFrames(): Promise<Array<Record<string, unknown>>>;
  createSessionRequests(): Promise<CreateSessionPayload[]>;
};

function cloneSession(session: SessionInfo): SessionInfo {
  return {
    ...session,
    view: {
      ...session.view,
      card: {
        ...session.view.card,
      },
      log_tabs: session.view.log_tabs ? [...session.view.log_tabs] : undefined,
      info_extras: session.view.info_extras ? [...session.view.info_extras] : undefined,
    },
  };
}

function makeViewUpdateFrame(
  sessions: SessionInfo[],
  activeSessionID?: string | null,
): ViewUpdateFrame {
  const frame: ViewUpdateFrame = {
    k: "v",
    sessions: sessions.map(cloneSession),
  };
  if (activeSessionID !== undefined) {
    frame.activeSessionID = activeSessionID;
  }
  return frame;
}

export async function installFakeBackend(
  page: Page,
  options: FakeBackendOptions,
): Promise<FakeBackend> {
  const ticket = options.ticket ?? "ticket-test";
  let sessions = options.sessions.map(cloneSession);
  const createSessionRequests: CreateSessionPayload[] = [];

  await page.addInitScript(() => {
    type ClientRecord = { raw: string; parsed: Record<string, unknown> | null };
    type ServerRecord = { raw: string; parsed: Record<string, unknown> | null };
    type SocketHarness = {
      sockets: FakeWebSocket[];
      sent: ClientRecord[];
      emitted: ServerRecord[];
      emit(frame: ServerFrame): void;
    };
    const isOpenGatewaySocket = (socket: { readyState: number; url?: string | null }): boolean => {
      if (socket.readyState !== 1 || !socket.url) return false;
      try {
        const parsed = new URL(socket.url, "http://127.0.0.1");
        return parsed.pathname === "/ws" && parsed.searchParams.get("ticket") !== null;
      } catch {
        return false;
      }
    };

    class FakeWebSocket {
      static readonly CONNECTING = 0;
      static readonly OPEN = 1;
      static readonly CLOSING = 2;
      static readonly CLOSED = 3;

      readonly url: string;
      readyState = FakeWebSocket.CONNECTING;
      onopen: ((ev: Event) => void) | null = null;
      onmessage: ((ev: MessageEvent<string>) => void) | null = null;
      onclose: ((ev: Event) => void) | null = null;
      onerror: ((ev: Event) => void) | null = null;

      constructor(url: string) {
        this.url = url;
        harness.sockets.push(this);
        queueMicrotask(() => {
          if (this.readyState !== FakeWebSocket.CONNECTING) return;
          this.readyState = FakeWebSocket.OPEN;
          this.onopen?.(new Event("open"));
        });
      }

      send(data: string): void {
        const raw = String(data);
        let parsed: Record<string, unknown> | null = null;
        try {
          parsed = JSON.parse(raw) as Record<string, unknown>;
        } catch {
          parsed = null;
        }
        harness.sent.push({ raw, parsed });
        if (parsed?.k === "s" && typeof parsed.reqId === "string") {
          queueMicrotask(() => {
            this.onmessage?.(
              new MessageEvent("message", {
                data: JSON.stringify({ k: "r", reqId: parsed?.reqId }),
              }),
            );
          });
        }
      }

      close(): void {
        if (this.readyState === FakeWebSocket.CLOSED) return;
        this.readyState = FakeWebSocket.CLOSED;
        queueMicrotask(() => {
          this.onclose?.(new Event("close"));
        });
      }

      addEventListener(): void {}
      removeEventListener(): void {}
      dispatchEvent(): boolean {
        return true;
      }
    }

    const harness: SocketHarness = {
      sockets: [],
      sent: [],
      emitted: [],
      emit(frame: ServerFrame): void {
        const raw = JSON.stringify(frame);
        let parsed: Record<string, unknown> | null = null;
        try {
          parsed = JSON.parse(raw) as Record<string, unknown>;
        } catch {
          parsed = null;
        }
        this.emitted.push({ raw, parsed });
        for (const socket of this.sockets) {
          if (!isOpenGatewaySocket(socket)) continue;
          socket.onmessage?.(new MessageEvent("message", { data: raw }));
        }
      },
    };

    Object.defineProperty(globalThis, "WebSocket", {
      configurable: true,
      writable: true,
      value: FakeWebSocket,
    });
    (globalThis as typeof globalThis & { __agentGridHarness?: SocketHarness }).__agentGridHarness =
      harness;
  });

  await page.route("**/api/ws-ticket", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ticket }),
    });
  });

  await page.route("**/api/session-config", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(options.sessionConfig),
    });
  });

  await page.route("**/api/sessions", async (route) => {
    if (route.request().method() !== "POST") {
      await route.fulfill({ status: 405, body: "method not allowed" });
      return;
    }
    const raw = route.request().postData() ?? "{}";
    const body = JSON.parse(raw) as CreateSessionPayload;
    createSessionRequests.push(body);

    const createdSessionId = options.createdSessionId ?? "session-new";
    const createdSession = cloneSession({
      id: createdSessionId,
      project: body.project,
      command: body.command,
      created_at: "2026-07-05T00:00:00Z",
      view: {
        card: { title: "Browser smoke" },
        status: "running",
      },
    });
    sessions = [...sessions, createdSession];

    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        id: createdSessionId,
        project: body.project,
        command: body.command,
        created_at: "2026-07-05T00:00:00Z",
      }),
    });

    await page.evaluate(
      ({ frame }) => {
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: { emit(frame: ServerFrame): void };
          }
        ).__agentGridHarness;
        harness?.emit(frame as ServerFrame);
      },
      { frame: makeViewUpdateFrame(sessions) },
    );
  });

  return {
    async waitForSocketOpen() {
      await page.waitForFunction(() => {
        const isOpenGatewaySocket = (socket: { readyState: number; url?: string | null }): boolean => {
          if (socket.readyState !== 1 || !socket.url) return false;
          try {
            const parsed = new URL(socket.url, "http://127.0.0.1");
            return parsed.pathname === "/ws" && parsed.searchParams.get("ticket") !== null;
          } catch {
            return false;
          }
        };
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: { sockets: Array<{ readyState: number; url?: string | null }> };
          }
        ).__agentGridHarness;
        return Boolean(harness?.sockets.some((socket) => isOpenGatewaySocket(socket)));
      });
    },
    async emit(frame: ServerFrame) {
      await page.evaluate(
        ({ frame }) => {
          const harness = (
            globalThis as typeof globalThis & {
              __agentGridHarness?: { emit(frame: ServerFrame): void };
            }
          ).__agentGridHarness;
          harness?.emit(frame as ServerFrame);
        },
        { frame },
      );
    },
    async emittedFrames() {
      return page.evaluate(() => {
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: {
              emitted: Array<{ raw: string; parsed: Record<string, unknown> | null }>;
            };
          }
        ).__agentGridHarness;
        return harness?.emitted.map((entry) => entry.parsed ?? { raw: entry.raw }) ?? [];
      });
    },
    async sentFrames() {
      return page.evaluate(() => {
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: {
              sent: Array<{ raw: string; parsed: Record<string, unknown> | null }>;
            };
          }
        ).__agentGridHarness;
        return harness?.sent.map((entry) => entry.parsed ?? { raw: entry.raw }) ?? [];
      });
    },
    async createSessionRequests() {
      return [...createSessionRequests];
    },
  };
}

export function makeSessionInfo(input: {
  id: string;
  project: string;
  command: string;
  title: string;
  status: NonNullable<SessionInfo["view"]["status"]>;
  workspace?: string;
}): SessionInfo {
  return {
    id: input.id,
    project: input.project,
    command: input.command,
    workspace: input.workspace,
    created_at: "2026-07-05T00:00:00Z",
    view: {
      card: { title: input.title },
      status: input.status,
    },
  };
}
