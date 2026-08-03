import { cleanup, fireEvent, render, screen } from "@testing-library/react";
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
});
