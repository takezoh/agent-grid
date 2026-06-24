import { vi } from "vitest";

// xterm.js relies on canvas/DOM features happy-dom does not implement.
// Mock the module so TerminalPane.test can render without exploding.
vi.mock("@xterm/xterm", () => {
  class FakeTerminal {
    onData(_cb: (d: string) => void) {
      return { dispose() {} };
    }
    onResize(_cb: (s: { cols: number; rows: number }) => void) {
      return { dispose() {} };
    }
    open(_el: HTMLElement) {}
    loadAddon(_a: unknown) {}
    write(_d: string) {}
    dispose() {}
  }
  return { Terminal: FakeTerminal };
});
vi.mock("@xterm/addon-fit", () => {
  class FakeFitAddon {
    fit() {}
  }
  return { FitAddon: FakeFitAddon };
});
vi.mock("@xterm/xterm/css/xterm.css", () => ({}));

// ---------------------------------------------------------------------------
// ResizeObserver mock (ADR 0034)
// happy-dom does not implement ResizeObserver. We provide a minimal mock that
// lets tests manually trigger resize callbacks via globalThis.__triggerResize.
// ---------------------------------------------------------------------------
type ResizeCallback = (entries: ResizeObserverEntry[]) => void;

const _roInstances = new Map<Element, ResizeCallback>();

class MockResizeObserver {
  private _cb: ResizeCallback;
  private _target: Element | null = null;

  constructor(cb: ResizeCallback) {
    this._cb = cb;
  }

  observe(target: Element) {
    this._target = target;
    _roInstances.set(target, this._cb);
  }

  disconnect() {
    if (this._target) {
      _roInstances.delete(this._target);
      this._target = null;
    }
  }
}

// triggerResizeImpl(target, entries) — manually fire the observer callback
// registered for `target`. Exposed on globalThis as `__triggerResize` so test
// files can call it without per-call `as unknown as { ... }` casts. The
// observer mock does not actually inspect entries, so the type is intentionally
// loose to let tests pass partial entry shapes for documentation purposes.
function triggerResizeImpl(target: Element, entries: unknown[] = []) {
  const cb = _roInstances.get(target);
  if (cb) cb(entries as ResizeObserverEntry[]);
}

// Augment globalThis so tests can call `globalThis.__triggerResize(...)` typed.
declare global {
  // eslint-disable-next-line no-var
  var __triggerResize: (target: Element, entries?: unknown[]) => void;
}

globalThis.ResizeObserver = MockResizeObserver as unknown as typeof ResizeObserver;
globalThis.__triggerResize = triggerResizeImpl;

// ---------------------------------------------------------------------------
// requestAnimationFrame synchronous mock (ADR 0034)
// happy-dom's rAF does not flush synchronously. We replace it with an
// implementation that runs callbacks immediately (synchronous flush) so that
// scheduleFit tests can assert fit.fit() is called after a single rAF tick.
// ---------------------------------------------------------------------------
let _rafIdCounter = 0;

globalThis.requestAnimationFrame = (cb: FrameRequestCallback): number => {
  const id = ++_rafIdCounter;
  cb(performance.now());
  return id;
};

globalThis.cancelAnimationFrame = (_id: number) => {
  // no-op: callbacks have already fired synchronously
};
