import { describe, expect, it, vi } from "vitest";
import { IdleQuitController } from "../src/main/idle-quit.js";

describe("IdleQuitController", () => {
  it("quits after idle with zero windows", () => {
    let open = 0;
    const quit = vi.fn();
    let tick: (() => void) | null = null;
    const ctrl = new IdleQuitController({
      idleMs: 100,
      openCount: () => open,
      onQuit: quit,
      setTimeout: (fn) => {
        tick = fn as () => void;
        return 1 as unknown as ReturnType<typeof setTimeout>;
      },
      clearTimeout: () => {
        tick = null;
      },
    });

    ctrl.onWindowsChanged(); // zero windows → arm
    expect(quit).not.toHaveBeenCalled();
    tick?.();
    expect(quit).toHaveBeenCalledTimes(1);
  });

  it("cancels idle when a window opens", () => {
    let open = 0;
    const quit = vi.fn();
    let tick: (() => void) | null = null;
    const ctrl = new IdleQuitController({
      idleMs: 100,
      openCount: () => open,
      onQuit: quit,
      setTimeout: (fn) => {
        tick = fn as () => void;
        return 1 as unknown as ReturnType<typeof setTimeout>;
      },
      clearTimeout: () => {
        tick = null;
      },
    });

    ctrl.onWindowsChanged();
    open = 1;
    ctrl.onWindowsChanged();
    expect(tick).toBeNull();
    expect(quit).not.toHaveBeenCalled();
  });
});
