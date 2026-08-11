import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
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
// unsubscribe and lifecycle-desired outbound (upstreamSessionID, :242), and
// the mirror-image inbound output rewrite (:305). Pinning the 5th crossing
// here consolidates the boundary onto one owner instead of teaching a sixth
// call site about fleet keys. The outcome-level counterpart that does NOT
// presume this location is e2e/real-stack/terminal-input.spec.ts, which only
// asserts that SPA keystrokes reach the PTY.

class FakeWS {
  static instances: FakeWS[] = [];
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
    this.onclose?.();
  }
  open() {
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

    conn.send({ k: "i", d: "ls\r", sessionId: FLEET_KEY });

    const input = sentFrames(ws).find((frame) => frame.k === "i");
    expect(input).toBeDefined();
    // Counterexample this rejects: shipping the compound fleet key, which the
    // gateway cannot resolve to a session (silent WriteRaw drop).
    expect(input?.sessionId).toBe("s1");
  });

  it("puts the upstream session id on the wire for terminal resize frames", async () => {
    const { conn, ws } = await openFleetConnection();

    conn.send({ k: "r", cols: 100, rows: 30, sessionId: FLEET_KEY });

    const resize = sentFrames(ws).find((frame) => frame.k === "r");
    expect(resize).toBeDefined();
    expect(resize?.sessionId).toBe("s1");
  });

  it("leaves bare (non-fleet) session ids untouched", async () => {
    const { conn, ws } = await openFleetConnection();

    conn.send({ k: "i", d: "x", sessionId: "s1" });

    const input = sentFrames(ws).find((frame) => frame.k === "i");
    expect(input?.sessionId).toBe("s1");
  });
});
