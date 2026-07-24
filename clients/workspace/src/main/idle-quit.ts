/**
 * Main stays alive with zero windows to serve the control pipe, then
 * exits after idle timeout (plan §4.2: 5 minutes default).
 */

export interface IdleQuitOptions {
  /** Idle duration with zero open windows before quit (default 5 min). */
  idleMs?: number;
  /** Returns current open window count. */
  openCount: () => number;
  /** Invoked when idle timeout fires. */
  onQuit: () => void;
  /** Injectable clock for tests. */
  now?: () => number;
  setTimeout?: (fn: () => void, ms: number) => ReturnType<typeof setTimeout>;
  clearTimeout?: (id: ReturnType<typeof setTimeout>) => void;
}

export class IdleQuitController {
  private readonly idleMs: number;
  private readonly openCount: () => number;
  private readonly onQuit: () => void;
  private readonly now: () => number;
  private readonly setTimeoutFn: (fn: () => void, ms: number) => ReturnType<typeof setTimeout>;
  private readonly clearTimeoutFn: (id: ReturnType<typeof setTimeout>) => void;
  private timer: ReturnType<typeof setTimeout> | null = null;
  private quitFired = false;

  constructor(opts: IdleQuitOptions) {
    this.idleMs = opts.idleMs ?? 5 * 60 * 1000;
    this.openCount = opts.openCount;
    this.onQuit = opts.onQuit;
    this.now = opts.now ?? Date.now;
    this.setTimeoutFn = opts.setTimeout ?? setTimeout;
    this.clearTimeoutFn = opts.clearTimeout ?? clearTimeout;
  }

  /** Call after open/close window mutations. */
  onWindowsChanged(): void {
    if (this.quitFired) return;
    if (this.openCount() > 0) {
      this.clear();
      return;
    }
    this.arm();
  }

  dispose(): void {
    this.clear();
  }

  private arm(): void {
    this.clear();
    this.timer = this.setTimeoutFn(() => {
      this.timer = null;
      if (this.openCount() === 0 && !this.quitFired) {
        this.quitFired = true;
        this.onQuit();
      }
    }, this.idleMs);
  }

  private clear(): void {
    if (this.timer !== null) {
      this.clearTimeoutFn(this.timer);
      this.timer = null;
    }
  }
}
