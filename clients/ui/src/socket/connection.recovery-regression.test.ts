import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { Connection } from "./connection";

class RecoveryWS {
  onopen: (() => void) | null = null;
  onmessage: ((event: { data: string }) => void) | null = null;
  onclose: (() => void) | null = null;
  onerror: (() => void) | null = null;

  send(): void {}
  close(): void {
    this.onclose?.();
  }
}

describe("Connection daemon recovery regressions", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
  });

  it("bounds automatic daemon reconnects, supports manual retry, and clears only on hello", async () => {
    const sockets: RecoveryWS[] = [];
    const conn = new Connection({
      ticketEndpoint: "/api/ws-ticket",
      wsUrl: () => "ws://gateway/ws",
      bearerToken: "token",
      fetchFn: vi.fn(async () => ({
        ok: true,
        status: 200,
        json: async () => ({ ticket: "ticket" }),
      })) as unknown as typeof fetch,
      wsFactory: () => {
        const socket = new RecoveryWS();
        sockets.push(socket);
        return socket as unknown as WebSocket;
      },
      sleep: async () => {},
    });

    await conn.start();
    for (let outage = 0; outage < 17; outage += 1) {
      await vi.waitFor(() => expect(sockets.length).toBeGreaterThan(outage));
      const socket = sockets[outage];
      socket?.onopen?.();
      socket?.onmessage?.({
        data: JSON.stringify({ k: "c", data: "daemon-disconnected" }),
      });
      socket?.onclose?.();
    }

    await vi.waitFor(() => expect(useDaemonStore.getState().status).toBe("closed"));
    expect(sockets).toHaveLength(17);

    conn.retry();
    await vi.waitFor(() => expect(sockets).toHaveLength(18));
    const recovered = sockets[17];
    recovered?.onopen?.();
    expect(useDaemonStore.getState().daemonDisconnected).toBe(true);
    recovered?.onmessage?.({
      data: JSON.stringify({
        k: "h",
        sessions: [],
        features: [],
        serverTime: 1,
      }),
    });

    expect(useDaemonStore.getState()).toMatchObject({
      status: "open",
      daemonDisconnected: false,
    });
  });
});
