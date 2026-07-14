import { create } from "zustand";
import type { ActivityEvent, ViewUpdateFrame } from "../wire/server";

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
};

export type DrawerTab = "viewer" | "diff" | "tree";

export type WorkspaceRootHandlePin = {
  frameGeneration: number;
  resolvedRootPath: string;
};

export type DrawerTarget = {
  sessionId: string;
  path: string;
  kind: FileEventKind;
};

export type WorkspaceActivityState = {
  /** Turn-aggregated rows keyed by session id. */
  rowsBySession: Record<string, TurnRow[]>;
  /** Last accepted sequence per session for gap / out-of-order detection. */
  lastSequenceBySession: Record<string, number>;
  /** Active session id used for cross-session guard on ingest. */
  scopedSessionId: string | null;
  /** True when WS dropped and reconnect is pending. */
  transportDegraded: boolean;
  /** Drawer UI state — lives here so rail + drawer share one source. */
  drawerOpen: boolean;
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

  setScopedSession: (sessionId: string | null) => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  applyActivityEvents: (sessionId: string, events: ActivityEvent[]) => void;
  setTransportDegraded: (degraded: boolean) => void;
  openDrawerFromRow: (target: DrawerTarget, initialTab?: DrawerTab) => void;
  openDrawerTree: (sessionId: string) => void;
  closeDrawer: () => void;
  setDrawerTab: (tab: DrawerTab) => void;
  setPinnedHandle: (handle: WorkspaceRootHandlePin | null) => void;
  clearStale: (path: string) => void;
  reloadDrawerContent: () => void;
  toggleRowExpanded: (path: string) => void;
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

export function rowKey(path: string): string {
  return path;
}

// ---------------------------------------------------------------------------
// Reducers
// ---------------------------------------------------------------------------

const STALE_COALESCE_MS = 500;

let lastStaleTouchAt = 0;

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
  };
}

const initialState = {
  rowsBySession: {} as Record<string, TurnRow[]>,
  lastSequenceBySession: {} as Record<string, number>,
  scopedSessionId: null as string | null,
  transportDegraded: false,
  drawerOpen: false,
  drawerTab: "viewer" as DrawerTab,
  drawerTarget: null as DrawerTarget | null,
  pinnedHandle: null as WorkspaceRootHandlePin | null,
  stalePaths: {} as Record<string, true>,
  staleAnnounceSeq: 0,
  lastStaleAnnouncePath: null as string | null,
  reloadGeneration: 0,
  expandedRows: new Set<string>() as ReadonlySet<string>,
};

export const useWorkspaceActivityStore = create<WorkspaceActivityState>()((set, get) => ({
  ...initialState,

  setScopedSession: (sessionId) => set({ scopedSessionId: sessionId }),

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

  setTransportDegraded: (degraded) => set({ transportDegraded: degraded }),

  openDrawerFromRow: (target, initialTab) => {
    const tab =
      initialTab ??
      (target.kind === "edit" ? "diff" : target.kind === "delete" ? "viewer" : "viewer");
    const resolvedTab = target.kind === "edit" ? "diff" : tab;
    set({
      drawerOpen: true,
      drawerTarget: target,
      drawerTab: target.kind === "delete" ? "viewer" : resolvedTab,
      pinnedHandle: null,
      stalePaths: {},
      lastStaleAnnouncePath: null,
    });
  },

  openDrawerTree: (sessionId) => {
    set({
      drawerOpen: true,
      drawerTarget: null,
      drawerTab: "tree",
      pinnedHandle: null,
      stalePaths: {},
      lastStaleAnnouncePath: null,
      scopedSessionId: sessionId,
    });
  },

  closeDrawer: () =>
    set({
      drawerOpen: false,
      drawerTarget: null,
      pinnedHandle: null,
      stalePaths: {},
      lastStaleAnnouncePath: null,
    }),

  setDrawerTab: (tab) => set({ drawerTab: tab }),

  setPinnedHandle: (handle) => set({ pinnedHandle: handle }),

  clearStale: (path) =>
    set((s) => {
      if (!s.stalePaths[path]) return s;
      const next = { ...s.stalePaths };
      delete next[path];
      return { stalePaths: next, lastStaleAnnouncePath: null };
    }),

  reloadDrawerContent: () =>
    set((s) => {
      const path = s.drawerTarget?.path;
      const nextStale = { ...s.stalePaths };
      if (path) delete nextStale[path];
      return {
        stalePaths: nextStale,
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

  reset: () =>
    set(() => ({
      ...initialState,
      expandedRows: new Set<string>(),
    })),
}));
