import { beforeEach, describe, expect, it } from "vitest";
import type { ActivityEvent } from "../../wire/server";
import { selectDrawerStale, selectTurnRows, useWorkspaceActivityStore } from "../workspaceActivity";

function turnRow(overrides: Partial<Extract<ActivityEvent, { type: "turn_row" }>> = {}) {
  return {
    type: "turn_row" as const,
    session_id: "s1",
    sequence: 1,
    turn_id: "t1",
    path: "src/foo.ts",
    kind: "read" as const,
    count: 1,
    events: [{ path: "src/foo.ts", kind: "read" as const }],
    ...overrides,
  };
}

let seqCounter = 0;
function nextSeq(): number {
  seqCounter += 1;
  return seqCounter;
}

function midTouch(overrides: Partial<Extract<ActivityEvent, { type: "mid_turn_touch" }>> = {}) {
  return {
    type: "mid_turn_touch" as const,
    session_id: "s1",
    sequence: nextSeq(),
    path: "src/foo.ts",
    tool_call_id: "tc1",
    ...overrides,
  };
}

describe("workspaceActivity store", () => {
  beforeEach(() => {
    seqCounter = 0;
    useWorkspaceActivityStore.getState().reset();
    useWorkspaceActivityStore.getState().setScopedSession("s1");
  });

  it("applies turn_row events into session rows", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [turnRow()]);
    const rows = selectTurnRows(useWorkspaceActivityStore.getState(), "s1");
    expect(rows).toHaveLength(1);
    expect(rows[0]?.path).toBe("src/foo.ts");
    expect(rows[0]?.count).toBe(1);
  });

  it("marks drawer target stale on mid_turn_touch", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [midTouch()]);
    const state = useWorkspaceActivityStore.getState();
    expect(selectDrawerStale(state, "src/foo.ts")).toBe(true);
  });

  it("coalesces rapid mid_turn_touch for the same path", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "edit",
    });
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [midTouch(), midTouch()]);
    expect(useWorkspaceActivityStore.getState().staleAnnounceSeq).toBe(1);
  });

  it("discards cross-session payloads", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s2", [turnRow({ session_id: "s2" })]);
    expect(selectTurnRows(useWorkspaceActivityStore.getState(), "s2")).toHaveLength(0);
  });

  it("discards out-of-order events and flags transport degraded", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [turnRow({ sequence: 3 })]);
    expect(useWorkspaceActivityStore.getState().transportDegraded).toBe(true);
    expect(selectTurnRows(useWorkspaceActivityStore.getState(), "s1")).toHaveLength(0);
  });

  it("reload clears stale and bumps reload generation", () => {
    useWorkspaceActivityStore.getState().openDrawerFromRow({
      sessionId: "s1",
      path: "src/foo.ts",
      kind: "read",
    });
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [midTouch()]);
    const before = useWorkspaceActivityStore.getState().reloadGeneration;
    useWorkspaceActivityStore.getState().reloadDrawerContent();
    const after = useWorkspaceActivityStore.getState();
    expect(after.reloadGeneration).toBe(before + 1);
    expect(selectDrawerStale(after, "src/foo.ts")).toBe(false);
  });

  it("reconnect backfill ingests events after transport degraded clears", () => {
    useWorkspaceActivityStore.getState().applyActivityEvents("s1", [turnRow({ sequence: 1 })]);
    useWorkspaceActivityStore
      .getState()
      .applyActivityEvents("s1", [turnRow({ sequence: 5, path: "gap.ts" })]);
    expect(useWorkspaceActivityStore.getState().transportDegraded).toBe(true);
    useWorkspaceActivityStore.getState().setTransportDegraded(false);
    useWorkspaceActivityStore
      .getState()
      .applyActivityEvents("s1", [turnRow({ sequence: 6, path: "backfill.ts" })]);
    const rows = selectTurnRows(useWorkspaceActivityStore.getState(), "s1");
    expect(rows.some((r) => r.path === "backfill.ts")).toBe(true);
    expect(useWorkspaceActivityStore.getState().transportDegraded).toBe(false);
  });

  it("applyViewUpdate ingests activity_events from frame", () => {
    useWorkspaceActivityStore.getState().applyViewUpdate({
      k: "v",
      sessions: [],
      activity_session_id: "s1",
      activity_events: [turnRow()],
    });
    expect(selectTurnRows(useWorkspaceActivityStore.getState(), "s1")).toHaveLength(1);
  });
});
