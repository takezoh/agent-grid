import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import type { FleetHealthView } from "../fleet/session-protocol";
import { useDaemonStore } from "../store/daemon";
import { StatusBar } from "./StatusBar";

const degraded: FleetHealthView = {
  hostInstanceId: "host-1",
  revision: 1,
  delivery: "live",
  servers: [
    {
      serverId: "a",
      serverLabel: "A",
      enabled: true,
      observation: { kind: "connected", observedAt: "2026-08-03T00:00:00.000Z" },
    },
    {
      serverId: "b",
      serverLabel: "B",
      enabled: true,
      observation: {
        kind: "unavailable",
        observedAt: "2026-08-03T00:00:00.000Z",
        reason: "http_error",
      },
    },
    {
      serverId: "c",
      serverLabel: "C",
      enabled: false,
      observation: { kind: "disabled" },
    },
  ],
};

describe("StatusBar (FR-023 / UAC-010)", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    useDaemonStore.getState().reset();
    window.agentGridWorkspace = undefined;
  });

  it("renders status_line on the left when present", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    render(<StatusBar statusLine="Running task" />);
    expect(screen.getByText("Running task")).toBeTruthy();
  });

  it("stays visible when status_line is absent and connection is nominal, showing 'connected'", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    const { container } = render(<StatusBar />);
    expect(container.querySelector("[data-role='status-bar']")).not.toBeNull();
    expect(screen.getByLabelText("connection state")?.textContent).toBe("connected");
  });

  it("shows connection state on the right when transport is degraded", () => {
    useDaemonStore.setState({ status: "reconnecting", daemonDisconnected: false });
    render(<StatusBar />);
    expect(screen.getByLabelText("connection state")?.textContent).toBe("reconnecting to server…");
  });

  it("shows both status_line and connection state when both apply", () => {
    useDaemonStore.setState({ status: "reconnecting", daemonDisconnected: false });
    render(<StatusBar statusLine="Syncing changes" />);
    expect(screen.getByText("Syncing changes")).toBeTruthy();
    expect(screen.getByLabelText("connection state")?.textContent).toBe("reconnecting to server…");
  });

  it("offers an explicit retry after bounded automatic recovery stops", () => {
    const onRetry = vi.fn();
    useDaemonStore.setState({ status: "closed", daemonDisconnected: true });
    render(<StatusBar onRetry={onRetry} />);
    screen.getByRole("button", { name: "Retry" }).click();
    expect(onRetry).toHaveBeenCalledOnce();
  });

  it("summarizes a partial hosted fleet outage without counting disabled servers", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    render(<StatusBar health={degraded} onOpenConnections={() => undefined} />);
    expect(screen.getByText("1/2 servers unavailable")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Connections" })).toBeTruthy();
  });

  it("does not report a fleet with an incompatible server as nominally connected", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    const firstServer = degraded.servers[0];
    if (!firstServer) throw new Error("expected degraded fixture to include a first server");
    const health: FleetHealthView = {
      ...degraded,
      servers: [
        firstServer,
        {
          serverId: "b",
          serverLabel: "B",
          enabled: true,
          observation: {
            kind: "incompatible_protocol",
            observedAt: "2026-08-03T00:00:00.000Z",
            reason: "protocol_mismatch",
            detail: "incompatible protocol: client protocol-3, server protocol-1",
          },
        },
      ],
    };
    render(<StatusBar health={health} onOpenConnections={() => undefined} />);
    const connectionState = screen.getByLabelText("connection state");
    expect(connectionState.dataset.nominal).toBe("false");
    expect(connectionState.textContent).toContain("incompatible");
    expect(connectionState.textContent).not.toBe("connected");
  });

  it("keeps Retry reachable with no session and focuses landmark when Retry disappears", () => {
    // rejects counterexample: session-only Retry or focus stolen by non-Retry transitions
    const onRetry = vi.fn(() => {
      useDaemonStore.setState({ status: "connecting", daemonDisconnected: false });
    });
    useDaemonStore.setState({
      status: "closed",
      daemonDisconnected: true,
      sessions: [],
      activeSessionID: null,
    });
    const { rerender } = render(<StatusBar onRetry={onRetry} health={null} />);
    const retry = screen.getByRole("button", { name: "Retry" });
    retry.focus();
    expect(document.activeElement).toBe(retry);
    fireEvent.click(retry);
    rerender(<StatusBar onRetry={onRetry} health={null} />);
    // After closed→connecting, Retry unmounts; landmark receives focus.
    useDaemonStore.setState({ status: "connecting", daemonDisconnected: false });
    rerender(<StatusBar onRetry={onRetry} health={null} />);
    const landmark = screen.getByLabelText("session status");
    expect(landmark.tabIndex).toBe(-1);
    expect(document.activeElement).toBe(landmark);
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
  });

  it("does not steal focus when Retry disappears after focus moved elsewhere", () => {
    // rejects counterexample: retryWasFocused stays true after blur
    useDaemonStore.setState({
      status: "closed",
      daemonDisconnected: true,
      sessions: [],
      activeSessionID: null,
    });
    const { rerender } = render(
      <>
        <button type="button">Elsewhere</button>
        <StatusBar onRetry={() => undefined} health={null} />
      </>,
    );
    const retry = screen.getByRole("button", { name: "Retry" });
    const elsewhere = screen.getByRole("button", { name: "Elsewhere" });
    retry.focus();
    expect(document.activeElement).toBe(retry);
    fireEvent.blur(retry, { relatedTarget: elsewhere });
    elsewhere.focus();
    expect(document.activeElement).toBe(elsewhere);
    // Retry remains mounted while focus is elsewhere, then disappears.
    act(() => {
      useDaemonStore.setState({ status: "connecting", daemonDisconnected: false });
    });
    rerender(
      <>
        <button type="button">Elsewhere</button>
        <StatusBar onRetry={() => undefined} health={null} />
      </>,
    );
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(document.activeElement).toBe(elsewhere);
  });

  it("keeps polite live text as a sibling of Retry", () => {
    useDaemonStore.setState({ status: "closed", daemonDisconnected: true });
    render(<StatusBar onRetry={() => undefined} />);
    const status = screen.getByRole("status", { name: "connection state" });
    const retry = screen.getByRole("button", { name: "Retry" });
    expect(status.contains(retry)).toBe(false);
    expect(status.parentElement?.contains(retry)).toBe(true);
  });

  it("renders ticking elapsed next to status_line", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });

    render(<StatusBar statusLine="Running task" statusChangedAt="2026-06-19T23:59:55Z" />);
    expect(screen.getByLabelText("elapsed").textContent).toBe("5s");

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByLabelText("elapsed").textContent).toBe("7s");
  });

  it("status-bar has 26px height contract in view.css", async () => {
    const fs = await import("node:fs/promises");
    const path = await import("node:path");
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "view.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/\.status-bar\s*\{[\s\S]*?height:\s*26px/);
    expect(source).toMatch(/color:\s*var\(--text-faint\)/);
  });

  it("does not create listConfigs polling or timers", async () => {
    const listConfigs = vi.fn(async () => []);
    window.agentGridWorkspace = {
      sessionFleet: {
        listConfigs,
        create: vi.fn(),
        subscribeHealth: vi.fn(() => () => undefined),
      },
    };
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    render(<StatusBar health={degraded} onOpenConnections={() => undefined} />);
    await act(async () => {
      await Promise.resolve();
    });
    expect(listConfigs).not.toHaveBeenCalled();
  });
});
