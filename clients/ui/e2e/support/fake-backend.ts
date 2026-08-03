import type { Page } from "@playwright/test";
import type { CreateSessionPayload } from "../../src/api/sessions";
import { PROTOCOL_VERSION } from "../../src/protocol-version";
import type { ServerFrame, SessionInfo, ViewUpdateFrame } from "../../src/wire/server";

export type FleetHealthSnapshotFixture = {
  schemaVersion: number;
  hostInstanceId: string;
  revision: number;
  servers: Array<{
    serverId: string;
    serverLabel: string;
    enabled: boolean;
    observation: Record<string, unknown>;
  }>;
};

export type FakeBackendOptions = {
  sessions: SessionInfo[];
  sessionConfig?: {
    project_roots: string[];
    project_paths: string[];
    projects: Array<{ path: string; isGit: boolean; isSandboxed: boolean }>;
    commands: string[];
    push_commands: string[];
  };
  ticket?: string;
  createdSessionId?: string;
  /** Optional multi-server host config projection. */
  hostServers?: Array<{
    serverId: string;
    displayName: string;
    enabled: boolean;
  }>;
  configPath?: string;
  /** Initial fleet health snapshot (host authority). */
  health?: FleetHealthSnapshotFixture | null;
  /** When true, gateway sockets open but never auto-send traffic. */
  suppressAutoHello?: boolean;
};

export type FakeBackend = {
  waitForSocketOpen(): Promise<void>;
  /** Number of gateway /ws sockets created so far (including closed). */
  gatewaySocketCount(): Promise<number>;
  waitForGatewaySocketCount(min: number): Promise<void>;
  emit(frame: ServerFrame): Promise<void>;
  emitHealth(snapshot: FleetHealthSnapshotFixture): Promise<void>;
  closeGatewaySockets(): Promise<void>;
  setRefuseGateway(value: boolean): Promise<void>;
  emittedFrames(): Promise<Array<Record<string, unknown>>>;
  sentFrames(): Promise<Array<Record<string, unknown>>>;
  createSessionRequests(): Promise<CreateSessionPayload[]>;
};

function cloneSession(session: SessionInfo): SessionInfo {
  return {
    ...session,
    status: {
      ...session.status,
      signal: session.status.signal ? { ...session.status.signal } : undefined,
    },
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

  await page.route("**/host/config", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        schema_version: 2,
        schemaVersion: 2,
        configPath: options.configPath ?? "/tmp/agent-grid-e2e/servers.json",
        config_path: options.configPath ?? "/tmp/agent-grid-e2e/servers.json",
        servers: options.hostServers ?? [
          { serverId: "test-server", displayName: "Test", enabled: true },
        ],
      }),
    });
  });

  await page.addInitScript(
    ({ initialSessions, initialHealth }) => {
      type ClientRecord = { raw: string; parsed: Record<string, unknown> | null };
      type ServerRecord = { raw: string; parsed: Record<string, unknown> | null };
      type SocketHarness = {
        sockets: FakeWebSocket[];
        sent: ClientRecord[];
        emitted: ServerRecord[];
        fleetSessions: SessionInfo[];
        healthSnapshot: FleetHealthSnapshotFixture | null;
        /** When true, new gateway sockets open then immediately close (no hello). */
        refuseGateway: boolean;
        emit(frame: ServerFrame): void;
        emitFleet(): void;
        emitHealth(snapshot: FleetHealthSnapshotFixture): void;
        closeGatewaySockets(): void;
        setRefuseGateway(value: boolean): void;
      };
      const isOpenGatewaySocket = (socket: {
        readyState: number;
        url?: string | null;
      }): boolean => {
        if (socket.readyState !== 1 || !socket.url) return false;
        try {
          const parsed = new URL(socket.url, "http://127.0.0.1");
          return parsed.pathname.endsWith("/ws") && parsed.searchParams.get("ticket") !== null;
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
            const path = new URL(this.url, "http://127.0.0.1").pathname;
            if (path === "/host/fleet") {
              harness.emitFleet();
            }
            if (path === "/host/fleet/health" && harness.healthSnapshot) {
              harness.emitHealth(harness.healthSnapshot);
            }
            if (harness.refuseGateway && isOpenGatewaySocket(this)) {
              queueMicrotask(() => this.close());
            }
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
          // The real gateway acks both legacy {k:"s"}/{k:"u"} and current
          // lifecycle-v2 {k:"ld"} frames with {k:"r"}.
          // so the React subscribeWithRetry / unsubscribeOnce promises resolve.
          // Without acking "u" here, TerminalSubscriptionController's reconcile
          // loop hangs forever awaiting the old session's unsubscribe response
          // whenever the active session switches, and the new session never
          // gets subscribed.
          if (
            (parsed?.k === "s" || parsed?.k === "u" || parsed?.k === "ld") &&
            typeof parsed.reqId === "string"
          ) {
            queueMicrotask(() => {
              this.onmessage?.(
                new MessageEvent("message", {
                  data: JSON.stringify({ k: "r", reqId: parsed?.reqId }),
                }),
              );
              if (parsed?.k === "ld" && parsed.correlation) {
                harness.emit({
                  k: "lo",
                  correlation: parsed.correlation,
                  status: "applied",
                } as ServerFrame);
              }
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
        fleetSessions: initialSessions,
        healthSnapshot: initialHealth,
        refuseGateway: false,
        setRefuseGateway(value: boolean): void {
          this.refuseGateway = value;
        },
        emitFleet(): void {
          const snapshot = {
            revision: this.emitted.length + 1,
            members: [
              {
                serverId: "test-server",
                serverLabel: "Test",
                generation: 1,
                state: "connected",
                sessions: this.fleetSessions,
              },
            ],
            sessions: this.fleetSessions.map((session) => ({
              ref: { serverId: "test-server", sessionId: session.id },
              serverLabel: "Test",
              session,
              stale: false,
            })),
          };
          for (const socket of this.sockets) {
            if (
              socket.readyState === FakeWebSocket.OPEN &&
              new URL(socket.url, "http://127.0.0.1").pathname === "/host/fleet"
            ) {
              socket.onmessage?.(new MessageEvent("message", { data: JSON.stringify(snapshot) }));
            }
          }
        },
        emitHealth(snapshot: FleetHealthSnapshotFixture): void {
          this.healthSnapshot = snapshot;
          for (const socket of this.sockets) {
            if (
              socket.readyState === FakeWebSocket.OPEN &&
              new URL(socket.url, "http://127.0.0.1").pathname === "/host/fleet/health"
            ) {
              socket.onmessage?.(new MessageEvent("message", { data: JSON.stringify(snapshot) }));
            }
          }
        },
        closeGatewaySockets(): void {
          for (const socket of [...this.sockets]) {
            if (!isOpenGatewaySocket(socket)) continue;
            socket.close();
          }
        },
        emit(frame: ServerFrame): void {
          const raw = JSON.stringify(frame);
          let parsed: Record<string, unknown> | null = null;
          try {
            parsed = JSON.parse(raw) as Record<string, unknown>;
          } catch {
            parsed = null;
          }
          this.emitted.push({ raw, parsed });
          if (
            !Array.isArray(frame) &&
            (frame.k === "h" || frame.k === "v") &&
            frame.sessions !== undefined
          ) {
            this.fleetSessions = structuredClone(frame.sessions);
            this.emitFleet();
          }
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
      (
        globalThis as typeof globalThis & { __agentGridHarness?: SocketHarness }
      ).__agentGridHarness = harness;
    },
    {
      initialSessions: sessions,
      initialHealth: options.health ?? null,
    },
  );

  await page.route("**/api/ws-ticket", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ ticket }),
    });
  });

  await page.route("**/*/api/capabilities", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ protocolVersion: PROTOCOL_VERSION }),
    });
  });

  await page.route("**/*session-config*", async (route) => {
    const sessionConfig = options.sessionConfig ?? {
      project_roots: [],
      project_paths: [],
      projects: [],
      commands: [],
      push_commands: [],
    };
    const commandProfiles = sessionConfig.commands.map((command) => ({
      id: command,
      label: command,
      revision: `${command}-e2e`,
      parameters: [],
    }));
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        ...sessionConfig,
        base_command_profiles: commandProfiles,
        projects: sessionConfig.projects.map((project) => ({
          ...project,
          command_profiles: commandProfiles,
        })),
      }),
    });
  });

  await page.route("**/sessions", async (route) => {
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
      command: body.profile_id,
      created_at: "2026-07-05T00:00:00Z",
      status: { lifecycle: "active", human_input: false, failure: "none" },
      view: {
        card: { title: "Browser smoke" },
      },
    });
    sessions = [...sessions, createdSession];

    await route.fulfill({
      status: 201,
      contentType: "application/json",
      body: JSON.stringify({
        id: createdSessionId,
        project: body.project,
        command: body.profile_id,
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
        const isOpenGatewaySocket = (socket: {
          readyState: number;
          url?: string | null;
        }): boolean => {
          if (socket.readyState !== 1 || !socket.url) return false;
          try {
            const parsed = new URL(socket.url, "http://127.0.0.1");
            return parsed.pathname.endsWith("/ws") && parsed.searchParams.get("ticket") !== null;
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
    async gatewaySocketCount() {
      return page.evaluate(() => {
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: { sockets: Array<{ url?: string | null }> };
          }
        ).__agentGridHarness;
        if (!harness) return 0;
        return harness.sockets.filter((socket) => {
          if (!socket.url) return false;
          try {
            const parsed = new URL(socket.url, "http://127.0.0.1");
            return parsed.pathname.endsWith("/ws") && parsed.searchParams.get("ticket") !== null;
          } catch {
            return false;
          }
        }).length;
      });
    },
    async waitForGatewaySocketCount(min: number) {
      await page.waitForFunction(
        ({ minCount }) => {
          const harness = (
            globalThis as typeof globalThis & {
              __agentGridHarness?: { sockets: Array<{ url?: string | null }> };
            }
          ).__agentGridHarness;
          if (!harness) return false;
          const count = harness.sockets.filter((socket) => {
            if (!socket.url) return false;
            try {
              const parsed = new URL(socket.url, "http://127.0.0.1");
              return parsed.pathname.endsWith("/ws") && parsed.searchParams.get("ticket") !== null;
            } catch {
              return false;
            }
          }).length;
          return count >= minCount;
        },
        { minCount: min },
        { timeout: 90_000 },
      );
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
    async emitHealth(snapshot: FleetHealthSnapshotFixture) {
      await page.evaluate(
        ({ snapshot }) => {
          const harness = (
            globalThis as typeof globalThis & {
              __agentGridHarness?: {
                emitHealth(snapshot: FleetHealthSnapshotFixture): void;
              };
            }
          ).__agentGridHarness;
          harness?.emitHealth(snapshot);
        },
        { snapshot },
      );
    },
    async closeGatewaySockets() {
      await page.evaluate(() => {
        const harness = (
          globalThis as typeof globalThis & {
            __agentGridHarness?: { closeGatewaySockets(): void };
          }
        ).__agentGridHarness;
        harness?.closeGatewaySockets();
      });
    },
    async setRefuseGateway(value: boolean) {
      await page.evaluate(
        ({ value }) => {
          const harness = (
            globalThis as typeof globalThis & {
              __agentGridHarness?: { setRefuseGateway(value: boolean): void };
            }
          ).__agentGridHarness;
          harness?.setRefuseGateway(value);
        },
        { value },
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
  status: "running" | "waiting" | "idle" | "stopped" | "failed";
  workspace?: string;
}): SessionInfo {
  return {
    id: input.id,
    project: input.project,
    command: input.command,
    workspace: input.workspace,
    created_at: "2026-07-05T00:00:00Z",
    status:
      input.status === "running"
        ? { lifecycle: "active", human_input: false, failure: "none" }
        : input.status === "waiting"
          ? { lifecycle: "idle", human_input: true, failure: "none" }
          : input.status === "stopped"
            ? { lifecycle: "stopped", human_input: false, failure: "none" }
            : input.status === "failed"
              ? { lifecycle: "stopped", human_input: false, failure: "failed" }
              : { lifecycle: "idle", human_input: false, failure: "none" },
    view: {
      card: { title: input.title },
    },
  };
}
