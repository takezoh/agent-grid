/**
 * Sole BrowserWindow creation point (contract-b1-window-registry-dedup,
 * contract-migration-window-per-session-invariant, FR-MIG-01).
 *
 * Atomic focus-or-create: concurrent openSession(id) converges to exactly one window.
 * Close is a view-collapse only — never a session-stop signal
 * (contract-window-close-not-session-stop).
 *
 * Lint rule: no `new BrowserWindow` outside this file.
 */

export interface WindowHandle {
  readonly id: string;
  focus(): void;
  close(): void;
  isDestroyed(): boolean;
  getBounds(): { x: number; y: number; width: number; height: number };
  setBounds(b: { x: number; y: number; width: number; height: number }): void;
}

export interface WindowFactory {
  create(sessionId: string, bounds?: WindowBounds): WindowHandle;
}

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface WorkspaceStateV1 {
  schema_version: 1;
  windows: Record<string, WindowBounds>;
}

export interface StateStore {
  load(): WorkspaceStateV1 | null;
  save(state: WorkspaceStateV1): void;
}

/**
 * In-memory atomic registry. Production wires Electron BrowserWindow via WindowFactory.
 */
export class WindowRegistry {
  private readonly windows = new Map<string, WindowHandle>();
  /** In-flight create promises — serializes concurrent openSession for the same id. */
  private readonly inflight = new Map<string, Promise<WindowHandle>>();
  private readonly factory: WindowFactory;
  private readonly store: StateStore | null;

  constructor(factory: WindowFactory, store: StateStore | null = null) {
    this.factory = factory;
    this.store = store;
  }

  get openCount(): number {
    this.gc();
    return this.windows.size;
  }

  has(sessionId: string): boolean {
    this.gc();
    return this.windows.has(sessionId);
  }

  /**
   * Focus existing or create. Concurrent callers for the same id share one create.
   */
  async openSession(sessionId: string): Promise<WindowHandle> {
    if (!sessionId) {
      throw new Error("sessionId required");
    }
    this.gc();
    const existing = this.windows.get(sessionId);
    if (existing && !existing.isDestroyed()) {
      existing.focus();
      return existing;
    }

    const pending = this.inflight.get(sessionId);
    if (pending) {
      const w = await pending;
      w.focus();
      return w;
    }

    const createPromise = this.createAndRegister(sessionId);
    this.inflight.set(sessionId, createPromise);
    try {
      return await createPromise;
    } finally {
      this.inflight.delete(sessionId);
    }
  }

  /** Close window view only — does NOT signal session stop to the daemon. */
  closeSessionView(sessionId: string): void {
    const w = this.windows.get(sessionId);
    if (!w) return;
    this.persistBounds(sessionId, w);
    if (!w.isDestroyed()) {
      w.close();
    }
    this.windows.delete(sessionId);
  }

  listSessionIds(): string[] {
    this.gc();
    return [...this.windows.keys()];
  }

  /** Restore previously open sessions that still exist on the daemon. */
  async restoreIfPresent(
    sessionIds: string[],
    sessionExists: (id: string) => Promise<boolean>,
  ): Promise<void> {
    const state = this.store?.load() ?? null;
    for (const id of sessionIds) {
      if (!(await sessionExists(id))) continue;
      const bounds = state?.windows[id];
      // Force create path with bounds by temporarily storing preferred bounds on factory.
      await this.openSession(id);
      if (bounds) {
        const w = this.windows.get(id);
        w?.setBounds(bounds);
      }
    }
  }

  private async createAndRegister(sessionId: string): Promise<WindowHandle> {
    // Re-check after await gaps (another path may have registered).
    const existing = this.windows.get(sessionId);
    if (existing && !existing.isDestroyed()) {
      existing.focus();
      return existing;
    }
    const state = this.store?.load();
    const bounds = state?.windows[sessionId];
    const handle = this.factory.create(sessionId, bounds);
    this.windows.set(sessionId, handle);
    this.persistAll();
    return handle;
  }

  private persistBounds(sessionId: string, w: WindowHandle): void {
    if (!this.store || w.isDestroyed()) return;
    const state = this.store.load() ?? emptyState();
    state.windows[sessionId] = w.getBounds();
    this.store.save(state);
  }

  private persistAll(): void {
    if (!this.store) return;
    const state = emptyState();
    for (const [id, w] of this.windows) {
      if (!w.isDestroyed()) {
        state.windows[id] = w.getBounds();
      }
    }
    this.store.save(state);
  }

  private gc(): void {
    for (const [id, w] of this.windows) {
      if (w.isDestroyed()) {
        this.windows.delete(id);
      }
    }
  }
}

function emptyState(): WorkspaceStateV1 {
  return { schema_version: 1, windows: {} };
}

/**
 * Load workspace-state.json with schema-version fail-safe
 * (contract-workspace-state-schema-evolution).
 */
export function loadWorkspaceState(raw: unknown): WorkspaceStateV1 | null {
  if (raw === null || typeof raw !== "object") return null;
  const obj = raw as Record<string, unknown>;
  const version = obj.schema_version;
  if (version === 1) {
    const windows = obj.windows;
    if (windows === null || typeof windows !== "object") return emptyState();
    return { schema_version: 1, windows: windows as Record<string, WindowBounds> };
  }
  // Unknown / future version: refuse silent corruption; start empty.
  return null;
}
