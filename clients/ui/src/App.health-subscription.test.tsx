import { cleanup, render } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "./store/daemon";
import { useFrameMessagingStore } from "./store/frameMessaging";
import { useNotificationsStore } from "./store/notifications";
import { usePaletteStore } from "./store/palette";
import { useWorkspaceActivityStore } from "./store/workspaceActivity";

const healthSubscribeCalls: Array<{ cleanup: ReturnType<typeof vi.fn> }> = [];

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
        subscribeHealth: (_listener: (view: unknown) => void) => {
          const cleanupFn = vi.fn();
          healthSubscribeCalls.push({ cleanup: cleanupFn });
          return cleanupFn;
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
});
