import { describe, expect, it, vi } from "vitest";
import {
  WorkspaceDisplayController,
  centeredBounds,
  listDisplayChoices,
  resolveSavedDisplay,
  type DisplayScreenLike,
} from "../src/main/display-placement.js";
import type { StateStore, WorkspaceStateV3 } from "../src/main/window-registry.js";

const primary = {
  id: 1,
  label: "Built-in",
  bounds: { x: 0, y: 0, width: 1920, height: 1080 },
  workArea: { x: 0, y: 0, width: 1920, height: 1040 },
};
const secondary = {
  id: 2,
  label: "Wide display",
  bounds: { x: 1920, y: 0, width: 2560, height: 1440 },
  workArea: { x: 1920, y: 0, width: 2560, height: 1400 },
};

function fakeScreen(displays = [primary, secondary]): DisplayScreenLike {
  return {
    getAllDisplays: () => displays,
    getPrimaryDisplay: () => primary,
    getDisplayMatching: (bounds) => bounds.x >= 1920 ? secondary : primary,
    dipToScreenPoint: (point) => ({ x: point.x * 2, y: point.y * 2 }),
  };
}

function memoryStore(initial: WorkspaceStateV3 | null): StateStore & { saved: WorkspaceStateV3 | null } {
  return {
    saved: initial,
    load() { return this.saved ? structuredClone(this.saved) : null; },
    save(state) { this.saved = structuredClone(state); },
  };
}

describe("display placement", () => {
  it("contract-display-anchor-is-physical: projects physical anchors and primary identity", () => {
    expect(listDisplayChoices(fakeScreen())).toEqual([
      { id: "1", label: "Built-in", primary: true, anchor: { x: 1920, y: 1080 } },
      { id: "2", label: "Wide display", primary: false, anchor: { x: 6400, y: 1440 } },
    ]);
  });

  it("centers and clamps a new window in the last-used display", () => {
    const store = memoryStore({
      schema_version: 3,
      windows: {},
      last_display: { id: "2", bounds: secondary.bounds },
    });
    const controller = new WorkspaceDisplayController(store, fakeScreen());
    expect(controller.initialBounds(undefined, { width: 1200, height: 800 })).toEqual({
      x: 2600,
      y: 300,
      width: 1200,
      height: 800,
    });
    expect(centeredBounds({ width: 3000, height: 2000 }, secondary.workArea)).toEqual({
      x: 1920,
      y: 0,
      width: 2560,
      height: 1400,
    });
  });

  it("falls back by saved bounds and then primary when a display id disappears", () => {
    expect(resolveSavedDisplay({ id: "gone", bounds: secondary.bounds }, fakeScreen()).id).toBe(2);
    expect(resolveSavedDisplay(
      { id: "gone", bounds: { x: 9000, y: 0, width: 1000, height: 1000 } },
      fakeScreen(),
    ).id).toBe(1);
  });

  it("records the display after creation and after a moved event", () => {
    const store = memoryStore(null);
    const controller = new WorkspaceDisplayController(store, fakeScreen());
    let bounds = { x: 100, y: 100, width: 800, height: 600 };
    let moved: (() => void) | null = null;
    controller.track({
      isDestroyed: () => false,
      getBounds: () => bounds,
      on: (_event, listener) => { moved = listener; },
    });
    expect(store.saved?.last_display?.id).toBe("1");
    bounds = { ...bounds, x: 2200 };
    (moved as (() => void) | null)?.();
    expect(store.saved?.last_display?.id).toBe("2");
  });
});
