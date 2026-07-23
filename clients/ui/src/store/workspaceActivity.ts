import { create } from "zustand";
import type { ActivityActor, ActivityEvent, ViewUpdateFrame } from "../wire/server";

// ---------------------------------------------------------------------------
// Public types
// ---------------------------------------------------------------------------

export type FileEventKind = "read" | "create" | "edit" | "delete";

export type ActivityEventEntry = {
  path: string;
  kind: FileEventKind;
  tool_call_id?: string;
};

export type TurnRow = {
  turnId: string;
  path: string;
  kind: FileEventKind;
  count: number;
  events: ActivityEventEntry[];
  sequence: number;
  actor?: ActivityActor;
};

export type DrawerTab = "viewer" | "diff" | "tree";

export type MainMode = "terminal" | "workspace";

export type WorkspaceRootHandlePin = {
  sessionId: string;
  frameGeneration: number;
  resolvedRootPath: string;
};

export type WorkspaceRequestIdentity = {
  sessionId: string;
  epoch: number;
};

export type SessionSwitchError = "pending_target_disappeared" | "active_session_disappeared_dirty";

export type DrawerTarget = {
  sessionId: string;
  path: string;
  kind: FileEventKind;
};

export type DirtyBufferEntry = {
  path: string;
  ifUnmodifiedSince: string;
  dirty: boolean;
};

export type ConflictOutcome =
  | "no_conflict"
  | "background_touch_clean_buffer"
  | "background_touch_dirty_buffer"
  | "reconnect_mtime_differs";

export type ConflictResolution = "keep_mine" | "take_theirs" | "merge";

export type WorkspaceActivityState = {
  /** Turn-aggregated rows keyed by session id. */
  rowsBySession: Record<string, TurnRow[]>;
  /** Last accepted sequence per session for gap / out-of-order detection. */
  lastSequenceBySession: Record<string, number>;
  /** Active session id used for cross-session guard on ingest. */
  scopedSessionId: string | null;
  /** Monotonic identity for all asynchronous Workspace requests. */
  workspaceEpoch: number;
  /** Dirty-aware session selection waits here until the operator decides. */
  pendingSessionSwitchId: string | null;
  sessionSwitchError: SessionSwitchError | null;
  /** Dirty editor recovery after its owning session disappeared. */
  orphanedRecovery: boolean;
  /** True when WS dropped and reconnect is pending. */
  transportDegraded: boolean;
  /** Drawer UI state — lives here so rail + drawer share one source. */
  drawerOpen: boolean;
  /** Main-area mode. Independent of drawerOpen: switching back to the
      terminal hides the workspace WITHOUT closing it, so the open file,
      dirty buffer and editor state survive mode round-trips. */
  mainMode: MainMode;
  drawerTab: DrawerTab;
  drawerTarget: DrawerTarget | null;
  pinnedHandle: WorkspaceRootHandlePin | null;
  /** Paths marked stale for the open drawer target (mid-turn touch). */
  stalePaths: Record<string, true>;
  /** Monotonic counter to coalesce rapid stale aria-live announcements. */
  staleAnnounceSeq: number;
  lastStaleAnnouncePath: string | null;
  /** Reload generation — bumped to trigger viewer refetch. */
  reloadGeneration: number;
  /** Expanded row paths in the activity rail. */
  expandedRows: ReadonlySet<string>;
  /** Dirty editor buffers keyed by path. */
  dirtyBuffers: Record<string, DirtyBufferEntry>;
  /** Explicit conflict partition per path (reconnect mtime mismatch). */
  conflictByPath: Record<string, ConflictOutcome>;
  /** Bumped when transport reconnects to trigger mtime re-fetch for dirty buffers. */
  reconnectResyncGeneration: number;
  /** Workspace root disappeared while drawer open. */
  rootDisappeared: boolean;
  /** Unsaved-close warning dialog visible. */
  closeWarningOpen: boolean;
  /** Monotonic counter for aria-live announcements (all kinds). */
  liveAnnounceSeq: number;

  setScopedSession: (sessionId: string | null) => void;
  requestSessionSwitch: (sessionId: string | null) => boolean;
  cancelPendingSessionSwitch: () => void;
  discardPendingSessionSwitch: () => string | null;
  markPendingSessionMissing: () => void;
  markActiveSessionMissing: () => void;
  completeOrphanedRecovery: () => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  applyActivityEvents: (sessionId: string, events: ActivityEvent[]) => void;
  setTransportDegraded: (degraded: boolean) => void;
  openDrawerFromRow: (target: DrawerTarget, initialTab?: DrawerTab) => void;
  openDrawerTree: (sessionId: string) => void;
  closeDrawer: () => void;
  /** Switch the main-area mode without touching workspace session state. */
  setMainMode: (mode: MainMode) => void;
  requestCloseDrawer: () => void;
  confirmDiscardAndClose: () => void;
  cancelCloseWarning: () => void;
  setDrawerTab: (tab: DrawerTab) => void;
  setPinnedHandle: (handle: WorkspaceRootHandlePin | null) => void;
  clearStale: (path: string) => void;
  reloadDrawerContent: () => void;
  toggleRowExpanded: (path: string) => void;
  registerDirtyBuffer: (path: string, ifUnmodifiedSince: string) => void;
  setBufferDirty: (path: string, dirty: boolean) => void;
  clearDirtyBuffer: (path: string) => void;
  setConflictOutcome: (path: string, outcome: ConflictOutcome) => void;
  clearConflict: (path: string) => void;
  setRootDisappeared: (gone: boolean) => void;
  resolveConflict: (path: string, resolution: ConflictResolution) => void;
  reset: () => void;
};

// ---------------------------------------------------------------------------
// Selectors
// ---------------------------------------------------------------------------

const EMPTY_TURN_ROWS: TurnRow[] = [];

export function selectTurnRows(state: WorkspaceActivityState, sessionId: string | null): TurnRow[] {
  if (!sessionId) return EMPTY_TURN_ROWS;
  return state.rowsBySession[sessionId] ?? EMPTY_TURN_ROWS;
}

export function selectHasDirtyWorkspace(state: WorkspaceActivityState): boolean {
  return Object.values(state.dirtyBuffers).some((buffer) => buffer.dirty);
}

export function isWorkspaceRequestCurrent(identity: WorkspaceRequestIdentity): boolean {
  const state = useWorkspaceActivityStore.getState();
  return (
    !state.orphanedRecovery &&
    state.scopedSessionId === identity.sessionId &&
    state.workspaceEpoch === identity.epoch
  );
}

export function selectDrawerStale(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): boolean {
  if (!path) return false;
  return state.stalePaths[path] === true;
}

export function selectStaleAnnounceMessage(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): string | null {
  if (!path || !state.stalePaths[path]) return null;
  return `Workspace file ${path} may be stale. Reload to refresh.`;
}

export function selectBufferDirty(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): boolean {
  if (!path) return false;
  return state.dirtyBuffers[path]?.dirty === true;
}

export function selectConflictOutcome(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): ConflictOutcome {
  if (!path) return "no_conflict";
  const explicit = state.conflictByPath[path];
  if (explicit === "reconnect_mtime_differs") return explicit;
  if (state.stalePaths[path]) {
    if (state.dirtyBuffers[path]?.dirty) return "background_touch_dirty_buffer";
    return "background_touch_clean_buffer";
  }
  return explicit ?? "no_conflict";
}

export function selectConflictBannerVisible(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): boolean {
  const outcome = selectConflictOutcome(state, path);
  return outcome === "background_touch_dirty_buffer" || outcome === "reconnect_mtime_differs";
}

/** aria-live precedence: conflict > stale > close-warning > dirty */
export function selectAriaLiveMessage(
  state: WorkspaceActivityState,
  path: string | null | undefined,
): string | null {
  if (selectConflictBannerVisible(state, path)) {
    return `Workspace file ${path} has a write conflict. Choose keep-mine, take-theirs, or merge.`;
  }
  const staleMsg = selectStaleAnnounceMessage(state, path);
  if (staleMsg) return staleMsg;
  if (state.closeWarningOpen) {
    return "Unsaved changes will be lost if you close the drawer.";
  }
  if (selectBufferDirty(state, path)) {
    return `Workspace file ${path} has unsaved changes.`;
  }
  return null;
}

export function rowKey(path: string): string {
  return path;
}

function bufferKey(path: string): string {
  return path;
}

// ---------------------------------------------------------------------------
// Reducers
// ---------------------------------------------------------------------------

const STALE_COALESCE_MS = 500;

let lastStaleTouchAt = 0;

function upsertOperatorTouchRow(
  rows: TurnRow[],
  event: Extract<ActivityEvent, { type: "mid_turn_touch" }>,
): TurnRow[] {
  if (event.actor !== "operator") return rows;
  const kind: FileEventKind = event.kind ?? "edit";
  const turnId = `operator-${event.tool_call_id ?? event.sequence}`;
  const idx = rows.findIndex((r) => r.path === event.path && r.actor === "operator");
  const next: TurnRow = {
    turnId,
    path: event.path,
    kind,
    count: 1,
    events: [
      {
        path: event.path,
        kind,
        tool_call_id: event.tool_call_id,
      },
    ],
    sequence: event.sequence,
    actor: "operator",
  };
  if (idx >= 0) {
    const copy = [...rows];
    copy[idx] = next;
    return copy;
  }
  return [...rows, next];
}

function upsertTurnRow(
  rows: TurnRow[],
  event: Extract<ActivityEvent, { type: "turn_row" }>,
): TurnRow[] {
  const idx = rows.findIndex((r) => r.path === event.path && r.turnId === event.turn_id);
  const next: TurnRow = {
    turnId: event.turn_id,
    path: event.path,
    kind: event.kind,
    count: event.count,
    events: event.events.map((e) => ({
      path: e.path,
      kind: e.kind,
      tool_call_id: e.tool_call_id,
    })),
    sequence: event.sequence,
    actor: event.actor,
  };
  if (idx >= 0) {
    const copy = [...rows];
    copy[idx] = next;
    return copy;
  }
  return [...rows, next];
}

function applyMidTurnTouch(
  state: WorkspaceActivityState,
  event: Extract<ActivityEvent, { type: "mid_turn_touch" }>,
): Partial<WorkspaceActivityState> {
  const drawerPath = state.drawerTarget?.path;
  if (!state.drawerOpen || drawerPath !== event.path) {
    return {};
  }
  const now = Date.now();
  const coalesced =
    event.path === state.lastStaleAnnouncePath && now - lastStaleTouchAt < STALE_COALESCE_MS;
  lastStaleTouchAt = now;
  return {
    stalePaths: { ...state.stalePaths, [event.path]: true },
    staleAnnounceSeq: coalesced ? state.staleAnnounceSeq : state.staleAnnounceSeq + 1,
    lastStaleAnnouncePath: event.path,
    liveAnnounceSeq: coalesced ? state.liveAnnounceSeq : state.liveAnnounceSeq + 1,
  };
}

const initialState = {
  rowsBySession: {} as Record<string, TurnRow[]>,
  lastSequenceBySession: {} as Record<string, number>,
  scopedSessionId: null as string | null,
  workspaceEpoch: 0,
  pendingSessionSwitchId: null as string | null,
  sessionSwitchError: null as SessionSwitchError | null,
  orphanedRecovery: false,
  transportDegraded: false,
  drawerOpen: false,
  mainMode: "terminal" as MainMode,
  drawerTab: "viewer" as DrawerTab,
  drawerTarget: null as DrawerTarget | null,
  pinnedHandle: null as WorkspaceRootHandlePin | null,
  stalePaths: {} as Record<string, true>,
  staleAnnounceSeq: 0,
  lastStaleAnnouncePath: null as string | null,
  reloadGeneration: 0,
  expandedRows: new Set<string>() as ReadonlySet<string>,
  dirtyBuffers: {} as Record<string, DirtyBufferEntry>,
  conflictByPath: {} as Record<string, ConflictOutcome>,
  reconnectResyncGeneration: 0,
  rootDisappeared: false,
  closeWarningOpen: false,
  liveAnnounceSeq: 0,
};

function clearDrawerEditorState(): Partial<WorkspaceActivityState> {
  return {
    dirtyBuffers: {},
    conflictByPath: {},
    rootDisappeared: false,
    orphanedRecovery: false,
    closeWarningOpen: false,
  };
}

function switchWorkspaceSession(
  state: WorkspaceActivityState,
  sessionId: string | null,
): Partial<WorkspaceActivityState> {
  return {
    scopedSessionId: sessionId,
    workspaceEpoch: state.workspaceEpoch + 1,
    pendingSessionSwitchId: null,
    sessionSwitchError: null,
    orphanedRecovery: false,
    drawerTarget: null,
    drawerTab: "tree",
    pinnedHandle: null,
    stalePaths: {},
    staleAnnounceSeq: 0,
    lastStaleAnnouncePath: null,
    reloadGeneration: state.reloadGeneration + 1,
    expandedRows: new Set<string>(),
    ...clearDrawerEditorState(),
  };
}

export const useWorkspaceActivityStore = create<WorkspaceActivityState>()((set, get) => ({
  ...initialState,

  setScopedSession: (sessionId) =>
    set((state) =>
      state.scopedSessionId === sessionId ? state : switchWorkspaceSession(state, sessionId),
    ),

  requestSessionSwitch: (sessionId) => {
    const state = get();
    if (state.orphanedRecovery) return false;
    if (state.scopedSessionId === sessionId) return true;
    if (selectHasDirtyWorkspace(state)) {
      if (state.pendingSessionSwitchId !== sessionId || state.sessionSwitchError !== null) {
        set({ pendingSessionSwitchId: sessionId, sessionSwitchError: null });
      }
      return false;
    }
    set((current) => switchWorkspaceSession(current, sessionId));
    return true;
  },

  cancelPendingSessionSwitch: () => set({ pendingSessionSwitchId: null, sessionSwitchError: null }),

  discardPendingSessionSwitch: () => {
    const pending = get().pendingSessionSwitchId;
    if (pending === null) return null;
    set({
      pendingSessionSwitchId: null,
      sessionSwitchError: null,
      dirtyBuffers: {},
      conflictByPath: {},
      stalePaths: {},
      closeWarningOpen: false,
      rootDisappeared: false,
    });
    return pending;
  },

  markPendingSessionMissing: () =>
    set((state) => ({
      pendingSessionSwitchId: null,
      sessionSwitchError: "pending_target_disappeared",
      liveAnnounceSeq: state.liveAnnounceSeq + 1,
    })),

  markActiveSessionMissing: () =>
    set((state) => ({
      pendingSessionSwitchId: null,
      sessionSwitchError: "active_session_disappeared_dirty",
      orphanedRecovery: true,
      rootDisappeared: true,
      closeWarningOpen: false,
      liveAnnounceSeq: state.liveAnnounceSeq + 1,
    })),

  completeOrphanedRecovery: () =>
    set((state) => ({
      ...switchWorkspaceSession(state, null),
      orphanedRecovery: false,
    })),

  applyViewUpdate: (frame) => {
    if (!frame.activity_events?.length) return;
    const sessionId = frame.activity_session_id ?? get().scopedSessionId;
    if (!sessionId) return;
    get().applyActivityEvents(sessionId, frame.activity_events);
  },

  applyActivityEvents: (sessionId, events) => {
    const scoped = get().scopedSessionId;
    if (scoped !== null && sessionId !== scoped) {
      return;
    }
    set((s) => {
      let rows = s.rowsBySession[sessionId] ?? [];
      let lastSeq = s.lastSequenceBySession[sessionId] ?? 0;
      let patch: Partial<WorkspaceActivityState> = {};

      for (const event of events) {
        if (event.session_id !== sessionId) continue;
        if (event.sequence <= lastSeq) continue;
        if (event.sequence > lastSeq + 1) {
          patch.transportDegraded = true;
          lastSeq = event.sequence;
          continue;
        }
        lastSeq = event.sequence;

        if (event.type === "turn_row") {
          rows = upsertTurnRow(rows, event);
        } else if (event.type === "mid_turn_touch") {
          if (event.actor === "operator") {
            rows = upsertOperatorTouchRow(rows, event);
          }
          patch = { ...patch, ...applyMidTurnTouch({ ...s, ...patch }, event) };
        }
      }

      return {
        ...patch,
        rowsBySession: { ...s.rowsBySession, [sessionId]: rows },
        lastSequenceBySession: { ...s.lastSequenceBySession, [sessionId]: lastSeq },
        transportDegraded: patch.transportDegraded ?? s.transportDegraded,
      };
    });
  },

  setTransportDegraded: (degraded) =>
    set((s) => {
      const wasDegraded = s.transportDegraded;
      const reconnecting = wasDegraded && !degraded;
      return {
        transportDegraded: degraded,
        reconnectResyncGeneration: reconnecting
          ? s.reconnectResyncGeneration + 1
          : s.reconnectResyncGeneration,
      };
    }),

  openDrawerFromRow: (target, initialTab) => {
    const tab =
      initialTab ??
      (target.kind === "edit" ? "diff" : target.kind === "delete" ? "viewer" : "viewer");
    const resolvedTab = target.kind === "edit" ? "diff" : tab;
    const alreadyOpen = get().drawerOpen;
    set({
      drawerOpen: true,
      mainMode: "workspace",
      drawerTarget: target,
      drawerTab: target.kind === "delete" ? "viewer" : resolvedTab,
      // First open pins fresh state; navigating within an open workspace
      // session keeps the pinned handle and editor state (mode persistence).
      ...(alreadyOpen
        ? {}
        : {
            pinnedHandle: null,
            stalePaths: {},
            lastStaleAnnouncePath: null,
            ...clearDrawerEditorState(),
          }),
    });
  },

  openDrawerTree: (sessionId) => {
    if (get().drawerOpen) {
      // Workspace session already exists — re-entering the mode must not
      // reset the open file / buffers (mode persistence).
      set({ mainMode: "workspace", scopedSessionId: sessionId });
      return;
    }
    set({
      drawerOpen: true,
      mainMode: "workspace",
      drawerTarget: null,
      drawerTab: "tree",
      pinnedHandle: null,
      stalePaths: {},
      lastStaleAnnouncePath: null,
      scopedSessionId: sessionId,
      ...clearDrawerEditorState(),
    });
  },

  closeDrawer: () =>
    set({
      drawerOpen: false,
      mainMode: "terminal",
      drawerTarget: null,
      pinnedHandle: null,
      stalePaths: {},
      lastStaleAnnouncePath: null,
      ...clearDrawerEditorState(),
    }),

  setMainMode: (mode) => set({ mainMode: mode }),

  requestCloseDrawer: () => {
    const s = get();
    const path = s.drawerTarget?.path;
    if (path && s.dirtyBuffers[path]?.dirty) {
      set({
        closeWarningOpen: true,
        liveAnnounceSeq: s.liveAnnounceSeq + 1,
      });
      return;
    }
    get().closeDrawer();
  },

  confirmDiscardAndClose: () => {
    get().closeDrawer();
  },

  cancelCloseWarning: () => set({ closeWarningOpen: false }),

  setDrawerTab: (tab) => set({ drawerTab: tab }),

  setPinnedHandle: (handle) => set({ pinnedHandle: handle }),

  clearStale: (path) =>
    set((s) => {
      if (!s.stalePaths[path]) return s;
      const nextStale = { ...s.stalePaths };
      delete nextStale[path];
      const nextConflict = { ...s.conflictByPath };
      if (nextConflict[path] !== "reconnect_mtime_differs") {
        delete nextConflict[path];
      }
      return {
        stalePaths: nextStale,
        conflictByPath: nextConflict,
        lastStaleAnnouncePath: null,
      };
    }),

  reloadDrawerContent: () =>
    set((s) => {
      const path = s.drawerTarget?.path;
      const nextStale = { ...s.stalePaths };
      const nextDirty = { ...s.dirtyBuffers };
      const nextConflict = { ...s.conflictByPath };
      if (path) {
        delete nextStale[path];
        delete nextDirty[path];
        delete nextConflict[path];
      }
      return {
        stalePaths: nextStale,
        dirtyBuffers: nextDirty,
        conflictByPath: nextConflict,
        reloadGeneration: s.reloadGeneration + 1,
        lastStaleAnnouncePath: null,
      };
    }),

  toggleRowExpanded: (path) =>
    set((s) => {
      const next = new Set(s.expandedRows);
      if (next.has(path)) next.delete(path);
      else next.add(path);
      return { expandedRows: next };
    }),

  registerDirtyBuffer: (path, ifUnmodifiedSince) =>
    set((s) => ({
      dirtyBuffers: {
        ...s.dirtyBuffers,
        [bufferKey(path)]: { path, ifUnmodifiedSince, dirty: false },
      },
    })),

  setBufferDirty: (path, dirty) =>
    set((s) => {
      const key = bufferKey(path);
      const existing = s.dirtyBuffers[key];
      if (!existing) {
        return {
          dirtyBuffers: {
            ...s.dirtyBuffers,
            [key]: { path, ifUnmodifiedSince: "", dirty },
          },
          liveAnnounceSeq: dirty ? s.liveAnnounceSeq + 1 : s.liveAnnounceSeq,
        };
      }
      const wasDirty = existing.dirty;
      return {
        dirtyBuffers: {
          ...s.dirtyBuffers,
          [key]: { ...existing, dirty },
        },
        liveAnnounceSeq: dirty && !wasDirty ? s.liveAnnounceSeq + 1 : s.liveAnnounceSeq,
      };
    }),

  clearDirtyBuffer: (path) =>
    set((s) => {
      const key = bufferKey(path);
      if (!s.dirtyBuffers[key]) return s;
      const next = { ...s.dirtyBuffers };
      delete next[key];
      return { dirtyBuffers: next };
    }),

  setConflictOutcome: (path, outcome) =>
    set((s) => ({
      conflictByPath: { ...s.conflictByPath, [path]: outcome },
      liveAnnounceSeq:
        outcome === "background_touch_dirty_buffer" || outcome === "reconnect_mtime_differs"
          ? s.liveAnnounceSeq + 1
          : s.liveAnnounceSeq,
    })),

  clearConflict: (path) =>
    set((s) => {
      if (!s.conflictByPath[path]) return s;
      const next = { ...s.conflictByPath };
      delete next[path];
      return { conflictByPath: next };
    }),

  setRootDisappeared: (gone) => set({ rootDisappeared: gone }),

  resolveConflict: (path, resolution) => {
    const s = get();
    if (resolution === "take_theirs") {
      get().reloadDrawerContent();
      return;
    }
    if (resolution === "merge") {
      set({
        drawerTab: "diff",
        conflictByPath: (() => {
          const next = { ...s.conflictByPath };
          delete next[path];
          return next;
        })(),
        stalePaths: (() => {
          const next = { ...s.stalePaths };
          delete next[path];
          return next;
        })(),
      });
      return;
    }
    // keep_mine: clear conflict markers; caller saves without precondition
    set((state) => {
      const nextConflict = { ...state.conflictByPath };
      const nextStale = { ...state.stalePaths };
      delete nextConflict[path];
      delete nextStale[path];
      return { conflictByPath: nextConflict, stalePaths: nextStale };
    });
  },

  reset: () =>
    set(() => ({
      ...initialState,
      expandedRows: new Set<string>(),
    })),
}));
