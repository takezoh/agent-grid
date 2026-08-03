import { act, cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import type { FleetHealthView } from "./fleet/session-protocol";
import { useDaemonStore } from "./store/daemon";
import { useFrameMessagingStore } from "./store/frameMessaging";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";
import { useWorkspaceActivityStore } from "./store/workspaceActivity";

const healthSubscribeCalls: Array<{ cleanup: ReturnType<typeof vi.fn> }> = [];
const healthListeners: Array<(view: FleetHealthView) => void> = [];
let listConfigsCallCount = 0;
let listConfigsImpl: () => Promise<Array<Record<string, unknown>>> = () => new Promise(() => {});

vi.mock("./lib/platform", () => ({
  isMacPlatform: vi.fn(() => false),
}));

vi.mock("./fleet/provider", async () => {
  const actual = await vi.importActual<typeof import("./fleet/provider")>("./fleet/provider");
  return {
    ...actual,
    makeFleetProvider: () => {
      const provider = actual.makeFleetProvider(vi.fn() as typeof fetch);
      return {
        ...provider,
        subscribe: () => () => undefined,
        // Do not push snapshots into React state — this test only counts host
        // subscription lifecycle (Finding 12).
        subscribeHealth: (listener: (view: FleetHealthView) => void) => {
          const cleanupFn = vi.fn();
          healthSubscribeCalls.push({ cleanup: cleanupFn });
          healthListeners.push(listener);
          return cleanupFn;
        },
        listConfigs: () => {
          listConfigsCallCount += 1;
          return listConfigsImpl();
        },
        configPath: async () => null,
      };
    },
  };
});

import { App } from "./App";

describe("App health subscription singleton", () => {
  beforeEach(() => {
    healthSubscribeCalls.length = 0;
    healthListeners.length = 0;
    listConfigsCallCount = 0;
    listConfigsImpl = () => new Promise(() => {});
    vi.useFakeTimers();
    useDaemonStore.getState().reset();
    useFrameMessagingStore.getState().reset();
    useNotificationsStore.setState({ items: [] });
    useWorkspaceActivityStore.getState().reset();
    usePaletteStore.getState().close();
    vi.stubGlobal(
      "fetch",
      vi.fn(() => new Promise(() => {})),
    );
    window.location.hash = "#token=test";
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.unstubAllGlobals();
    window.location.hash = "";
    usePaletteStore.getState().close();
  });

  it("application lifecycle owns exactly one host health subscription and one cleanup", () => {
    // rejects counterexample: dependency/remount multiplication of subscribeHealth
    const first = render(<App />);
    expect(healthSubscribeCalls).toHaveLength(1);
    const firstSub = healthSubscribeCalls[0];
    expect(firstSub).toBeDefined();
    if (!firstSub) throw new Error("expected first health subscription");

    first.rerender(<App />);
    // Same fleet memo → effect deps unchanged → no additional host subscription.
    expect(healthSubscribeCalls).toHaveLength(1);
    expect(firstSub.cleanup).not.toHaveBeenCalled();

    first.unmount();
    expect(firstSub.cleanup).toHaveBeenCalledTimes(1);

    const beforeSecond = healthSubscribeCalls.length;
    const second = render(<App />);
    expect(healthSubscribeCalls.length).toBe(beforeSecond + 1);
    const secondSub = healthSubscribeCalls[healthSubscribeCalls.length - 1];
    expect(secondSub).toBeDefined();
    if (!secondSub) throw new Error("expected second health subscription");
    expect(secondSub.cleanup).not.toHaveBeenCalled();
    second.unmount();
    expect(secondSub.cleanup).toHaveBeenCalledTimes(1);
  });

  it("coalesces config rehydration on unavailable-to-connected recovery edges", async () => {
    listConfigsImpl = async () => {
      throw new Error("temporary config failure");
    };
    render(<App />);
    for (let i = 0; i < 5; i++) {
      await act(async () => Promise.resolve());
    }
    expect(listConfigsCallCount).toBe(1);
    expect(useDaemonStore.getState().sessionConfig).toBeNull();

    const listener = healthListeners[0];
    if (!listener) throw new Error("expected health listener");
    const health = (revision: number, kind: "unavailable" | "connected"): FleetHealthView => ({
      hostInstanceId: "host-a",
      revision,
      delivery: "live",
      servers: [
        {
          serverId: "a",
          serverLabel: "A",
          enabled: true,
          observation:
            kind === "connected"
              ? { kind, observedAt: "2026-08-03T00:00:10Z" }
              : {
                  kind,
                  observedAt: "2026-08-03T00:00:09Z",
                  reason: "connection_failed",
                },
        },
      ],
    });
    act(() => listener(health(1, "unavailable")));
    expect(listConfigsCallCount).toBe(1);

    let resolveRecovery: ((value: Array<Record<string, unknown>>) => void) | undefined;
    listConfigsImpl = () =>
      new Promise((resolve) => {
        resolveRecovery = resolve;
      });
    act(() => listener(health(2, "connected")));
    await act(async () => Promise.resolve());
    expect(listConfigsCallCount).toBe(2);

    // Connected revisions are not recovery edges, and even a second edge
    // coalesces into the still in-flight recovery request.
    act(() => {
      listener(health(3, "connected"));
      listener(health(4, "connected"));
      listener(health(5, "unavailable"));
      listener(health(6, "connected"));
    });
    await act(async () => Promise.resolve());
    expect(listConfigsCallCount).toBe(2);

    await act(async () => {
      resolveRecovery?.([
        {
          serverId: "a",
          serverLabel: "A",
          state: "connected",
          config: {
            base_command_profiles: [],
            projects: [],
            push_commands: ["/clear"],
          },
        },
      ]);
      await Promise.resolve();
    });

    expect(listConfigsCallCount).toBe(2);
    expect(useDaemonStore.getState().sessionConfig?.pushCommands).toEqual(["/clear"]);
  });
});
