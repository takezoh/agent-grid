/**
 * Sole BrowserWindow creation point (contract-b1-window-registry-dedup,
 * contract-migration-window-per-session-invariant, FR-MIG-01).
 *
 * Atomic focus-or-create: concurrent openSession({serverId, sessionId})
 * converges to exactly one window.
 * Close is a view-collapse only — never a session-stop signal
 * (contract-window-close-not-session-stop).
 *
 * Lint rule: no `new BrowserWindow` outside this file.
 */

import {
  assertSessionRef,
  sessionKey,
  type SessionRef,
} from "../shared/session-ref.js";

export interface WindowHandle {
  readonly id: string;
  focus(): void;
  close(): void;
  isDestroyed(): boolean;
  getBounds(): { x: number; y: number; width: number; height: number };
  setBounds(b: { x: number; y: number; width: number; height: number }): void;
}

export interface WindowFactory {
  create(session: SessionRef, bounds?: WindowBounds): WindowHandle;
}

export interface WindowBounds {
  x: number;
  y: number;
  width: number;
  height: number;
}

export interface WorkspaceStateV2 {
  schema_version: 2;
  windows: Record<string, Record<string, WindowBounds>>;
}

export interface StateStore {
  load(): WorkspaceStateV2 | null;
  save(state: WorkspaceStateV2): void;
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

  has(session: SessionRef): boolean {
    this.gc();
    return this.windows.has(sessionKey(session));
  }

  /**
   * Focus existing or create. Concurrent callers for the same id share one create.
   */
  async openSession(session: SessionRef): Promise<WindowHandle> {
    assertSessionRef(session);
    const key = sessionKey(session);
    this.gc();
    const existing = this.windows.get(key);
    if (existing && !existing.isDestroyed()) {
      existing.focus();
      return existing;
    }

    const pending = this.inflight.get(key);
    if (pending) {
      const w = await pending;
      w.focus();
      return w;
    }

    const createPromise = this.createAndRegister(session);
    this.inflight.set(key, createPromise);
    try {
      return await createPromise;
    } finally {
      this.inflight.delete(key);
    }
  }

  /** Close window view only — does NOT signal session stop to the daemon. */
  closeSessionView(session: SessionRef): void {
    const key = sessionKey(session);
    const w = this.windows.get(key);
    if (!w) return;
    this.persistBounds(session, w);
    if (!w.isDestroyed()) {
      w.close();
    }
    this.windows.delete(key);
  }

  listSessions(): SessionRef[] {
    this.gc();
    return [...this.windows.keys()].map((key) => {
      const [serverId, sessionId] = JSON.parse(key) as [string, string];
      return { serverId, sessionId };
    });
  }

  /** Restore previously open sessions that still exist on the daemon. */
  async restoreIfPresent(
    sessions: SessionRef[],
    sessionExists: (session: SessionRef) => Promise<boolean>,
  ): Promise<void> {
    const state = this.store?.load() ?? null;
    for (const session of sessions) {
      if (!(await sessionExists(session))) continue;
      const bounds = state?.windows[session.serverId]?.[session.sessionId];
      // Force create path with bounds by temporarily storing preferred bounds on factory.
      await this.openSession(session);
      if (bounds) {
        const w = this.windows.get(sessionKey(session));
        w?.setBounds(bounds);
      }
    }
  }

  private async createAndRegister(session: SessionRef): Promise<WindowHandle> {
    const key = sessionKey(session);
    // Re-check after await gaps (another path may have registered).
    const existing = this.windows.get(key);
    if (existing && !existing.isDestroyed()) {
      existing.focus();
      return existing;
    }
    const state = this.store?.load();
    const bounds = state?.windows[session.serverId]?.[session.sessionId];
    const handle = this.factory.create(session, bounds);
    this.windows.set(key, handle);
    this.persistAll();
    return handle;
  }

  private persistBounds(session: SessionRef, w: WindowHandle): void {
    if (!this.store || w.isDestroyed()) return;
    const state = this.store.load() ?? emptyState();
    state.windows[session.serverId] ??= {};
    state.windows[session.serverId]![session.sessionId] = w.getBounds();
    this.store.save(state);
  }

  private persistAll(): void {
    if (!this.store) return;
    const state = emptyState();
    for (const [key, w] of this.windows) {
      if (!w.isDestroyed()) {
        const [serverId, sessionId] = JSON.parse(key) as [string, string];
        state.windows[serverId] ??= {};
        state.windows[serverId]![sessionId] = w.getBounds();
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

function emptyState(): WorkspaceStateV2 {
  return { schema_version: 2, windows: {} };
}

/**
 * Load workspace-state.json with schema-version fail-safe
 * (contract-workspace-state-schema-evolution).
 */
export function loadWorkspaceState(raw: unknown): WorkspaceStateV2 | null {
  if (raw === null || typeof raw !== "object") return null;
  const obj = raw as Record<string, unknown>;
  const version = obj.schema_version;
  if (version === 2) {
    const windows = obj.windows;
    if (windows === null || typeof windows !== "object") return emptyState();
    return {
      schema_version: 2,
      windows: windows as Record<string, Record<string, WindowBounds>>,
    };
  }
  // Unknown / future version: refuse silent corruption; start empty.
  return null;
}
