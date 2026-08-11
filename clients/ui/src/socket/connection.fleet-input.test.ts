import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";
import { useSubscriptionStore } from "../store/subscriptions";
import { Connection } from "./connection";

// RED (debug-run-workspace-electron-terminal-dead / hypothesis-input-key-untranslated)
//
// Since 9ef53ddc the SPA keys sessions by the compound fleet key
// JSON.stringify([serverId, sessionId]). Connection.upstreamSessionID() strips
// that key back to the bare daemon id on the subscribe / unsubscribe /
// lifecycle-desired paths, and handleMessage() performs the mirror-image
// rewrite on inbound output frames. The outbound data-plane frames sent by
// TerminalPane (`{k:"i"}` keystrokes, `{k:"r"}` resize) go through
// Connection.send() verbatim, so the compound key reaches the gateway, which
// hands it to sess.WriteRaw() and drops it silently
// (services/server/api/gateway_lifecycle_inbound.go:120-146).
//
// The contract these tests pin: every outbound frame carrying a sessionId
// leaves the Connection with the upstream (bare) daemon id.
//
// Why the pin is on Connection and not on the frame builders: Connection is
// already the wire-boundary translator for 4 of the 5 crossings — subscribe,
// unsubscribe and lifecycle-desired outbound (upstreamSessionID), and the
// mirror-image inbound output rewrite. Pinning the 5th crossing here
// consolidates the boundary onto one owner instead of teaching a sixth call
// site about fleet keys. The outcome-level counterpart that does NOT presume
// this location is e2e/real-stack/terminal-input.spec.ts, which only asserts
// that SPA keystrokes reach the PTY.
//
// The three original witnesses are preserved; they now drive the intent API
// (sendTerminalInput / sendTerminalResize) rather than assembling a frame,
// because assembling one outside Connection is the thing that broke.

class FakeWS {
  static instances: FakeWS[] = [];
  // Mirrors the real WebSocket: sends before open() throw there, so a fake
  // without this would be more permissive than the thing it stands in for.
  readyState: number = WebSocket.CONNECTING;
  onopen: (() => void) | null = null;
  onmessage: ((ev: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;
  sent: string[] = [];
  constructor(public url: string) {
    FakeWS.instances.push(this);
  }
  send(d: string) {
    this.sent.push(d);
  }
  close() {
    this.readyState = WebSocket.CLOSED;
    this.onclose?.();
  }
  open() {
    this.readyState = WebSocket.OPEN;
    this.onopen?.();
  }
}

const FLEET_KEY = JSON.stringify(["srv-a", "s1"]);

async function openFleetConnection(): Promise<{ conn: Connection; ws: FakeWS }> {
  const fetchFn = vi.fn(async () => ({
    ok: true,
    status: 200,
    json: async () => ({ ticket: "tkt" }),
  })) as unknown as typeof fetch;
  const conn = new Connection({
    ticketEndpoint: "/api/ws-ticket",
    wsUrl: (t) => `ws://h/ws?ticket=${t}`,
    bearerToken: "tok",
    publishSessions: false,
    serverId: "srv-a",
    wsFactory: (u) => new FakeWS(u) as unknown as WebSocket,
    sleep: async () => {},
    fetchFn,
  });
  await conn.start();
  const ws = FakeWS.instances[0];
  if (!ws) throw new Error("expected socket");
  ws.open();
  return { conn, ws };
}

function sentFrames(ws: FakeWS): Array<Record<string, unknown>> {
  return ws.sent.map((raw) => JSON.parse(raw) as Record<string, unknown>);
}

describe("Connection outbound session-id translation (fleet-qualified keys)", () => {
  beforeEach(() => {
    FakeWS.instances = [];
    useDaemonStore.getState().reset();
    useSubscriptionStore.getState().reset();
  });

  it("puts the upstream session id on the wire for terminal input frames", async () => {
    const { conn, ws } = await openFleetConnection();

    conn.sendTerminalInput(FLEET_KEY, "ls\r");

    const input = sentFrames(ws).find((frame) => frame.k === "i");
    expect(input).toBeDefined();
    // Counterexample this rejects: shipping the compound fleet key, which the
    // gateway cannot resolve to a session (silent WriteRaw drop).
    expect(input?.sessionId).toBe("s1");
  });

  it("puts the upstream session id on the wire for terminal resize frames", async () => {
    const { conn, ws } = await openFleetConnection();

    conn.sendTerminalResize(FLEET_KEY, 100, 30);

    const resize = sentFrames(ws).find((frame) => frame.k === "r");
    expect(resize).toBeDefined();
    expect(resize?.sessionId).toBe("s1");
  });

  it("leaves bare (non-fleet) session ids untouched", async () => {
    const { conn, ws } = await openFleetConnection();

    conn.sendTerminalInput("s1", "x");

    const input = sentFrames(ws).find((frame) => frame.k === "i");
    expect(input?.sessionId).toBe("s1");
  });
});

// contract-dataplane-unresolved-drop.
//
// The data plane runs inside xterm's synchronous onData emitter, so it cannot
// throw. It also must not drop silently — silent drops are exactly what let
// this defect survive two repairs on the gateway side. So every drop is
// counted, and only the reason that means "internal contract violation"
// reaches the user: a server switch legitimately strands in-flight keystrokes
// addressed to the previous server, and toasting that would report normal
// operation as a failure.
describe("Connection data-plane drops", () => {
  beforeEach(() => {
    FakeWS.instances = [];
    useDaemonStore.getState().reset();
    useSubscriptionStore.getState().reset();
    useNotificationsStore.getState().clear();
  });

  it("counts a server mismatch without throwing, sending, or notifying", async () => {
    const { conn, ws } = await openFleetConnection();
    const other = JSON.stringify(["srv-b", "s9"]);

    for (let i = 0; i < 50; i++) {
      expect(() => conn.sendTerminalInput(other, "x")).not.toThrow();
    }

    expect(sentFrames(ws).filter((f) => f.k === "i")).toHaveLength(0);
    expect(conn.dataPlaneDrops["server-mismatch"]).toBe(50);
    // Counterexample this rejects: a toast during the window where App.tsx is
    // rebuilding the Connection for a newly selected server.
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("notifies once per session for an undecodable id while still counting each drop", async () => {
    const { conn, ws } = await openFleetConnection();

    for (let i = 0; i < 50; i++) conn.sendTerminalInput("not a session id!", "x");

    expect(sentFrames(ws).filter((f) => f.k === "i")).toHaveLength(0);
    expect(conn.dataPlaneDrops["decode-failed"]).toBe(50);
    expect(useNotificationsStore.getState().items).toHaveLength(1);
  });

  it("folds resize drops onto the same reason key as input drops", async () => {
    const { conn } = await openFleetConnection();

    conn.sendTerminalInput("not a session id!", "x");
    conn.sendTerminalResize("not a session id!", 80, 24);

    expect(conn.dataPlaneDrops["decode-failed"]).toBe(2);
    expect(useNotificationsStore.getState().items).toHaveLength(1);
  });

  it("does not record anything for a resolvable key", async () => {
    const { conn, ws } = await openFleetConnection();

    for (let i = 0; i < 50; i++) conn.sendTerminalInput(FLEET_KEY, "x");

    expect(sentFrames(ws).filter((f) => f.k === "i")).toHaveLength(50);
    expect(conn.dataPlaneDrops).toEqual({ "server-mismatch": 0, "decode-failed": 0 });
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("carries control sequences to the wire verbatim", async () => {
    const { conn, ws } = await openFleetConnection();
    const del = "\u001b[3~";

    conn.sendTerminalInput(FLEET_KEY, del);

    const input = sentFrames(ws).find((frame) => frame.k === "i");
    expect(input?.d).toBe(del);
  });
});
