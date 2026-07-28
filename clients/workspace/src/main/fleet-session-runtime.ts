import type { ServerConfig } from "./desktop-config.js";

export interface FleetSessionRef {
  serverId: string;
  sessionId: string;
}

export interface FleetMember {
  serverId: string;
  serverLabel: string;
  generation: number;
  state: "connecting" | "connected" | "reconnecting" | "unavailable";
  stale?: boolean;
  reason?: string;
  sessions: Array<Record<string, unknown>>;
}

export interface FleetSnapshot {
  revision: number;
  members: FleetMember[];
  sessions: Array<{
    ref: FleetSessionRef;
    serverLabel: string;
    session: Record<string, unknown>;
    stale: boolean;
  }>;
}

type Listener = (snapshot: FleetSnapshot) => void;
type FetchLike = typeof fetch;
type WebSocketLike = Pick<
  WebSocket,
  "close" | "addEventListener"
>;

export class FleetSessionRuntime {
  private snapshotValue: FleetSnapshot;
  private readonly listeners = new Set<Listener>();
  private readonly generations = new Map<string, number>();
  private readonly sockets = new Set<WebSocketLike>();
  private stopped = false;

  constructor(
    private servers: readonly ServerConfig[],
    private readonly fetchFn: FetchLike = fetch,
    private readonly webSocketFactory: (url: string) => WebSocketLike = (url) =>
      new WebSocket(url),
  ) {
    this.snapshotValue = project(0, servers.filter((server) => server.enabled).map((server) => ({
      serverId: server.id,
      serverLabel: server.display_name,
      generation: 0,
      state: "connecting",
      sessions: [],
    })));
  }

  start(): void {
    this.stopped = false;
    for (const server of this.servers) {
      if (server.enabled) void this.follow(server);
    }
  }

  stop(): void {
    this.stopped = true;
    for (const socket of this.sockets) socket.close();
    this.sockets.clear();
    this.listeners.clear();
  }

  subscribe(listener: Listener): () => void {
    this.listeners.add(listener);
    listener(this.snapshotValue);
    return () => this.listeners.delete(listener);
  }

  waitFor(ref: FleetSessionRef, timeoutMs = 10_000): Promise<boolean> {
    return new Promise((resolve) => {
      let settled = false;
      let unsubscribe = () => {};
      const finish = (value: boolean) => {
        if (settled) return;
        settled = true;
        clearTimeout(timer);
        unsubscribe();
        resolve(value);
      };
      const timer = setTimeout(() => finish(false), timeoutMs);
      unsubscribe = this.subscribe((snapshot) => {
        if (
          snapshot.sessions.some(
            (session) =>
              session.ref.serverId === ref.serverId && session.ref.sessionId === ref.sessionId,
          )
        ) {
          finish(true);
        }
      });
      if (settled) unsubscribe();
    });
  }

  replace(server: ServerConfig, generation: number, patch: Omit<FleetMember, "serverId" | "serverLabel" | "generation">): void {
    const index = this.snapshotValue.members.findIndex((member) => member.serverId === server.id);
    if (index < 0) return;
    const current = this.snapshotValue.members[index];
    if (generation < current.generation) return;
    const failed = patch.state !== "connected";
    const members = this.snapshotValue.members.slice();
    members[index] = {
      serverId: server.id,
      serverLabel: server.display_name,
      generation,
      ...patch,
      stale: failed && current.sessions.length > 0,
      sessions: failed ? current.sessions : patch.sessions,
    };
    this.snapshotValue = project(this.snapshotValue.revision + 1, members);
    for (const listener of this.listeners) listener(this.snapshotValue);
  }

  private async follow(server: ServerConfig): Promise<void> {
    while (!this.stopped) {
      const generation = (this.generations.get(server.id) ?? 0) + 1;
      this.generations.set(server.id, generation);
      try {
        await this.followGeneration(server, generation);
      } catch (error) {
        if (this.stopped) return;
        this.replace(server, generation, {
          state: "reconnecting",
          reason: error instanceof Error ? error.message : String(error),
          sessions: [],
        });
        await new Promise((resolve) => setTimeout(resolve, 1_000));
      }
    }
  }

  private async followGeneration(server: ServerConfig, generation: number): Promise<void> {
    const ticketResponse = await this.fetchFn(
      new URL("/api/ws-ticket", `${server.url.replace(/\/+$/, "")}/`),
      {
        method: "POST",
        headers: fleetHeaders(server),
        redirect: "manual",
      },
    );
    if (!ticketResponse.ok) throw new Error(`ws-ticket HTTP ${ticketResponse.status}`);
    const { ticket } = (await ticketResponse.json()) as { ticket?: unknown };
    if (typeof ticket !== "string" || !ticket) throw new Error("empty websocket ticket");
    const url = new URL("/ws", `${server.url.replace(/\/+$/, "")}/`);
    url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
    url.searchParams.set("ticket", ticket);
    url.searchParams.set("protocolVersion", "protocol-2");
    const socket = this.webSocketFactory(url.toString());
    this.sockets.add(socket);
    await new Promise<void>((resolve, reject) => {
      socket.addEventListener("message", (event: Event) => {
        const data = (event as MessageEvent).data;
        if (typeof data !== "string") return;
        let frame: { k?: unknown; sessions?: unknown };
        try {
          frame = JSON.parse(data) as typeof frame;
        } catch {
          return;
        }
        if ((frame.k === "h" || frame.k === "v") && Array.isArray(frame.sessions)) {
          this.replace(server, generation, {
            state: "connected",
            sessions: frame.sessions.filter(
              (session): session is Record<string, unknown> =>
                Boolean(session) && typeof session === "object",
            ),
          });
        }
      });
      socket.addEventListener("close", () => resolve());
      socket.addEventListener("error", () => reject(new Error("websocket failed")));
    });
    this.sockets.delete(socket);
    if (!this.stopped) throw new Error("websocket closed");
  }
}

function fleetHeaders(server: ServerConfig): Headers {
  const headers = new Headers({ "X-Agent-Grid-Protocol-Version": "protocol-2" });
  if (server.auth.mode === "bearer") {
    headers.set("Authorization", `Bearer ${server.auth.token}`);
  }
  return headers;
}

function project(revision: number, members: FleetMember[]): FleetSnapshot {
  const labelCounts = new Map<string, number>();
  for (const member of members) {
    labelCounts.set(member.serverLabel, (labelCounts.get(member.serverLabel) ?? 0) + 1);
  }
  return {
    revision,
    members,
    sessions: members.flatMap((member) =>
      member.sessions.flatMap((session) =>
        typeof session.id === "string" && session.id
          ? [{
              ref: { serverId: member.serverId, sessionId: session.id },
              serverLabel:
                labelCounts.get(member.serverLabel) === 1
                  ? member.serverLabel
                  : `${member.serverLabel} · ${member.serverId}`,
              session,
              stale: member.stale === true,
            }]
          : [],
      ),
    ),
  };
}
