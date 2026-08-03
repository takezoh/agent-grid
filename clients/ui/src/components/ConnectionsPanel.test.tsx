import { act, cleanup, fireEvent, render, screen, waitFor } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import {
  DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION,
  DELIVERY_LOST_BEFORE_CONFIG_RECOVERY,
} from "../fleet/connections-presentation";
import type { FleetHealthView } from "../fleet/session-protocol";
import { ConnectionsPanel } from "./ConnectionsPanel";

const health: FleetHealthView = {
  hostInstanceId: "host-1",
  revision: 2,
  delivery: "lost",
  servers: [
    {
      serverId: "a",
      serverLabel: "Alpha",
      enabled: true,
      observation: {
        kind: "health_unavailable",
        observedAt: "2026-08-03T00:00:10.000Z",
        reason: "delivery_lost",
        prior: { kind: "connected", observedAt: "2026-08-03T00:00:05.000Z" },
      },
    },
  ],
};

/** Dual failure: /host/config failed and health WS closed before first snapshot. */
const dualFailureHealth: FleetHealthView = {
  hostInstanceId: null,
  revision: null,
  delivery: "lost",
  servers: [],
};

describe("ConnectionsPanel", () => {
  afterEach(() => {
    cleanup();
    vi.unstubAllGlobals();
  });

  // rejects counterexample: Connections owns Retry or disables config path during failure
  it("shows server-qualified diagnosis and copyable config path without Retry", async () => {
    const writeText = vi.fn().mockResolvedValue(undefined);
    vi.stubGlobal("navigator", {
      ...navigator,
      clipboard: { writeText },
    });
    render(
      <ConnectionsPanel
        open
        health={health}
        configPath="/var/lib/agent-grid/servers.json"
        onClose={() => undefined}
      />,
    );
    expect(screen.getByRole("dialog", { name: "Connections" })).toBeTruthy();
    expect(screen.getByText("Alpha")).toBeTruthy();
    expect(screen.getByText("Health unavailable")).toBeTruthy();
    expect(screen.getByText("stale prior")).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(document.querySelector('[data-role="delivery-loss-diagnosis"]')).toBeNull();
    expect(screen.queryByText(DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION)).toBeNull();
    const copy = screen.getByRole("button", { name: "Copy configuration path" });
    fireEvent.click(copy);
    expect(writeText).toHaveBeenCalledWith("/var/lib/agent-grid/servers.json");
    expect(await screen.findByText("Copied")).toBeTruthy();
    expect(screen.getByText(/restart/i)).toBeTruthy();
  });

  it("falls back when navigator.clipboard is missing", async () => {
    const execCommand = vi.fn(() => true);
    vi.stubGlobal("navigator", { ...navigator, clipboard: undefined });
    Object.defineProperty(document, "execCommand", {
      configurable: true,
      writable: true,
      value: execCommand,
    });

    render(
      <ConnectionsPanel
        open
        health={health}
        configPath="/etc/agent-grid/servers.json"
        onClose={() => undefined}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Copy configuration path" }));
    expect(execCommand).toHaveBeenCalledWith("copy");
    expect(await screen.findByText("Copied")).toBeTruthy();
  });

  it("surfaces failure when writeText rejects without unhandled rejection", async () => {
    const writeText = vi.fn().mockRejectedValue(new Error("denied"));
    vi.stubGlobal("navigator", {
      ...navigator,
      clipboard: { writeText },
    });
    const unhandled: unknown[] = [];
    const onUnhandled = (reason: unknown) => {
      unhandled.push(reason);
    };
    process.on("unhandledRejection", onUnhandled);

    render(
      <ConnectionsPanel
        open
        health={health}
        configPath="/var/lib/agent-grid/servers.json"
        onClose={() => undefined}
      />,
    );
    fireEvent.click(screen.getByRole("button", { name: "Copy configuration path" }));
    expect(await screen.findByText("Could not copy path")).toBeTruthy();
    await Promise.resolve();
    process.off("unhandledRejection", onUnhandled);
    expect(unhandled).toEqual([]);
  });

  // rejects counterexample: dual failure (config + pre-snapshot health loss) opens blank panel
  it("explains zero-server delivery loss with recovery and configuration guidance", () => {
    render(
      <ConnectionsPanel
        open
        health={dualFailureHealth}
        configPath={null}
        onClose={() => undefined}
      />,
    );
    const dialog = screen.getByRole("dialog", { name: "Connections" });
    expect(dialog).toBeTruthy();
    const diagnosis = dialog.querySelector('[data-role="delivery-loss-diagnosis"]');
    expect(diagnosis).toBeTruthy();
    expect(diagnosis?.getAttribute("data-state")).toBe("health_unavailable");
    expect(screen.getByText("Health delivery lost")).toBeTruthy();
    expect(screen.getByText(DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION)).toBeTruthy();
    expect(screen.getByText(DELIVERY_LOST_BEFORE_CONFIG_RECOVERY)).toBeTruthy();
    // Recovery + settings guidance both mention configuration path recovery.
    expect(screen.getAllByText(/servers\.json/i).length).toBeGreaterThanOrEqual(1);
    expect(screen.getAllByText(/restart/i).length).toBeGreaterThanOrEqual(1);
    // No invented server row and no Retry ownership on Connections.
    expect(dialog.querySelectorAll("[data-server-id]")).toHaveLength(0);
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Copy configuration path" })).toBeNull();
    expect(dialog.textContent).not.toMatch(/Bearer |ticket=|gho_/i);
  });

  it("keeps normal empty live fleet free of delivery-loss copy", () => {
    render(
      <ConnectionsPanel
        open
        health={{
          hostInstanceId: "host-1",
          revision: 1,
          delivery: "live",
          servers: [],
        }}
        configPath="/etc/agent-grid/servers.json"
        onClose={() => undefined}
      />,
    );
    expect(screen.queryByText(DELIVERY_LOST_BEFORE_CONFIG_EXPLANATION)).toBeNull();
    expect(screen.queryByText(DELIVERY_LOST_BEFORE_CONFIG_RECOVERY)).toBeNull();
    expect(screen.getByRole("button", { name: "Copy configuration path" })).toBeTruthy();
  });

  const multiServerHealth: FleetHealthView = {
    hostInstanceId: "host-1",
    revision: 3,
    delivery: "live",
    servers: [
      {
        serverId: "a",
        serverLabel: "Alpha",
        enabled: true,
        observation: {
          kind: "unavailable",
          observedAt: "2026-08-03T00:00:10.000Z",
          reason: "connection_failed",
        },
      },
      {
        serverId: "b",
        serverLabel: "Beta",
        enabled: true,
        observation: {
          kind: "connected",
          observedAt: "2026-08-03T00:00:10.000Z",
        },
      },
      {
        serverId: "c",
        serverLabel: "Gamma",
        enabled: false,
        observation: { kind: "disabled" },
      },
    ],
  };

  // rejects counterexample: reconnect is a no-op / affects every server / ignores disabled
  it("renders server-qualified Reconnect only for enabled rows and invokes the callback", async () => {
    const onReconnect = vi.fn().mockResolvedValue(undefined);
    render(
      <ConnectionsPanel
        open
        health={multiServerHealth}
        onClose={() => undefined}
        onReconnect={onReconnect}
      />,
    );
    expect(screen.getByRole("button", { name: "Reconnect Alpha" })).toBeTruthy();
    expect(screen.getByRole("button", { name: "Reconnect Beta" })).toBeTruthy();
    expect(screen.queryByRole("button", { name: "Reconnect Gamma" })).toBeNull();
    expect(screen.queryByRole("button", { name: "Retry" })).toBeNull();

    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reconnect Alpha" }));
    });
    expect(onReconnect).toHaveBeenCalledTimes(1);
    expect(onReconnect).toHaveBeenCalledWith("a");
    expect(onReconnect).not.toHaveBeenCalledWith("b");
    expect(onReconnect).not.toHaveBeenCalledWith("c");
  });

  it("disables only the pending server action and clears pending on success", async () => {
    let resolveReconnect!: () => void;
    const onReconnect = vi.fn(
      () =>
        new Promise<void>((resolve) => {
          resolveReconnect = resolve;
        }),
    );
    render(
      <ConnectionsPanel
        open
        health={multiServerHealth}
        onClose={() => undefined}
        onReconnect={onReconnect}
      />,
    );
    const alpha = screen.getByRole("button", { name: "Reconnect Alpha" });
    const beta = screen.getByRole("button", { name: "Reconnect Beta" });
    await act(async () => {
      fireEvent.click(alpha);
    });
    expect(alpha).toHaveProperty("disabled", true);
    expect(alpha.getAttribute("data-pending")).toBe("true");
    expect(screen.getByText(/Reconnecting Alpha/)).toBeTruthy();
    // Peer action stays interactive — single-server pending only.
    expect(beta).toHaveProperty("disabled", false);
    // Single-flight: second click while pending does not re-enter.
    fireEvent.click(alpha);
    expect(onReconnect).toHaveBeenCalledTimes(1);

    await act(async () => {
      resolveReconnect();
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Reconnect Alpha" })).toHaveProperty(
        "disabled",
        false,
      );
    });
    expect(screen.queryByText(/Reconnecting Alpha/)).toBeNull();
  });

  it("retains the list and shows a credential-free failure with retry after rejection", async () => {
    const onReconnect = vi
      .fn()
      .mockRejectedValueOnce(new Error("unknown or disabled server"))
      .mockResolvedValueOnce(undefined);
    render(
      <ConnectionsPanel
        open
        health={multiServerHealth}
        onClose={() => undefined}
        onReconnect={onReconnect}
      />,
    );
    await act(async () => {
      fireEvent.click(screen.getByRole("button", { name: "Reconnect Alpha" }));
    });
    await waitFor(() => {
      const errorNode = document.querySelector('[data-role="reconnect-error"][data-server-id="a"]');
      expect(errorNode?.textContent).toBe("Reconnect failed for Alpha: unknown or disabled server");
      expect(errorNode?.textContent).not.toMatch(/secret-token|Bearer |gho_/i);
    });
    // List rows remain (not wiped).
    expect(document.querySelectorAll("[data-server-id]").length).toBeGreaterThanOrEqual(3);
    // Retry allowed after failure.
    const retry = screen.getByRole("button", { name: "Reconnect Alpha" });
    expect(retry).toHaveProperty("disabled", false);
    await act(async () => {
      fireEvent.click(retry);
    });
    expect(onReconnect).toHaveBeenCalledTimes(2);
  });

  it("tracks pending independently so concurrent reconnects stay single-flight per server", async () => {
    const resolvers = new Map<string, () => void>();
    const onReconnect = vi.fn(
      (serverId: string) =>
        new Promise<void>((resolve) => {
          resolvers.set(serverId, resolve);
        }),
    );
    render(
      <ConnectionsPanel
        open
        health={multiServerHealth}
        onClose={() => undefined}
        onReconnect={onReconnect}
      />,
    );
    const alpha = screen.getByRole("button", { name: "Reconnect Alpha" });
    const beta = screen.getByRole("button", { name: "Reconnect Beta" });

    await act(async () => {
      fireEvent.click(alpha);
    });
    await act(async () => {
      fireEvent.click(beta);
    });

    expect(onReconnect).toHaveBeenCalledTimes(2);
    expect(onReconnect).toHaveBeenNthCalledWith(1, "a");
    expect(onReconnect).toHaveBeenNthCalledWith(2, "b");
    expect(alpha).toHaveProperty("disabled", true);
    expect(beta).toHaveProperty("disabled", true);
    expect(alpha.getAttribute("data-pending")).toBe("true");
    expect(beta.getAttribute("data-pending")).toBe("true");
    expect(screen.getByText(/Reconnecting Alpha/)).toBeTruthy();
    expect(screen.getByText(/Reconnecting Beta/)).toBeTruthy();

    // Duplicate Alpha while both pending must be rejected (single-flight).
    fireEvent.click(alpha);
    expect(onReconnect).toHaveBeenCalledTimes(2);

    // Settle Beta only — Alpha remains disabled and single-flight.
    await act(async () => {
      resolvers.get("b")?.();
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Reconnect Beta" })).toHaveProperty(
        "disabled",
        false,
      );
    });
    expect(screen.getByRole("button", { name: "Reconnect Alpha" })).toHaveProperty(
      "disabled",
      true,
    );
    expect(screen.getByText(/Reconnecting Alpha/)).toBeTruthy();
    expect(screen.queryByText(/Reconnecting Beta/)).toBeNull();

    fireEvent.click(screen.getByRole("button", { name: "Reconnect Alpha" }));
    expect(onReconnect).toHaveBeenCalledTimes(2);

    await act(async () => {
      resolvers.get("a")?.();
    });
    await waitFor(() => {
      expect(screen.getByRole("button", { name: "Reconnect Alpha" })).toHaveProperty(
        "disabled",
        false,
      );
    });
    expect(screen.queryByText(/Reconnecting Alpha/)).toBeNull();
  });
});
