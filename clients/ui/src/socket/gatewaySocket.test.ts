import { describe, expect, it } from "vitest";
import { isGatewaySocketURL, isOpenGatewaySocket } from "./gatewaySocket";

describe("isGatewaySocketURL", () => {
  it("accepts the app gateway websocket path with a ticket", () => {
    expect(isGatewaySocketURL("ws://127.0.0.1:4173/ws?ticket=ticket-test")).toBe(true);
  });

  it("rejects the Vite HMR websocket path", () => {
    expect(isGatewaySocketURL("ws://127.0.0.1:4173/@vite/client")).toBe(false);
  });

  it("rejects websocket URLs without a ticket query", () => {
    expect(isGatewaySocketURL("ws://127.0.0.1:4173/ws")).toBe(false);
  });
});

describe("isOpenGatewaySocket", () => {
  it("requires both OPEN state and a gateway URL", () => {
    expect(
      isOpenGatewaySocket({
        readyState: 1,
        url: "ws://127.0.0.1:4173/@vite/client",
      }),
    ).toBe(false);

    expect(
      isOpenGatewaySocket({
        readyState: 0,
        url: "ws://127.0.0.1:4173/ws?ticket=ticket-test",
      }),
    ).toBe(false);

    expect(
      isOpenGatewaySocket({
        readyState: 1,
        url: "ws://127.0.0.1:4173/ws?ticket=ticket-test",
      }),
    ).toBe(true);
  });
});
