import { describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
  type FleetSessionRow,
  parseSessionKey,
  resolveSessionRoute,
  sessionKey,
} from "./session-identity";

function row(overrides: Partial<FleetSessionRow> = {}): FleetSessionRow {
  const base: SessionInfo = {
    id: "",
    project: "p",
    command: "c",
    created_at: "2026-08-11T00:00:00Z",
    status: "idle",
    view: { card: { title: "t", body: "" } },
  };
  return {
    ...base,
    id: sessionKey({ serverId: "srv-a", sessionId: "s1" }),
    serverId: "srv-a",
    upstreamSessionId: "s1",
    ...overrides,
  };
}

describe("fleet key encoding", () => {
  it("round-trips a ref", () => {
    const ref = { serverId: "srv-a", sessionId: "s1" };
    expect(parseSessionKey(sessionKey(ref))).toEqual(ref);
  });

  it("returns null for a bare upstream id rather than guessing", () => {
    expect(parseSessionKey("s1")).toBeNull();
  });

  it("returns null for a shape that is not a two-string tuple", () => {
    expect(parseSessionKey(JSON.stringify(["srv-a"]))).toBeNull();
    expect(parseSessionKey(JSON.stringify(["srv-a", "s1", "extra"]))).toBeNull();
    expect(parseSessionKey(JSON.stringify(["srv-a", 1]))).toBeNull();
    expect(parseSessionKey(JSON.stringify({ serverId: "srv-a" }))).toBeNull();
  });
});

describe("resolveSessionRoute precedence", () => {
  // The row wins over the id string. Without this order a caller holding the
  // authoritative row can still re-derive a route from the id — the exact
  // pattern that produced this defect class twice (9ef53ddc, 525b3091).
  it("prefers row authority over decoding the id, even when they disagree", () => {
    const conflicting = row({
      id: sessionKey({ serverId: "srv-b", sessionId: "stale" }),
      serverId: "srv-a",
      upstreamSessionId: "s1",
    });
    expect(resolveSessionRoute(conflicting)).toEqual({
      ok: true,
      serverId: "srv-a",
      upstreamSessionId: "s1",
      source: "row",
    });
  });

  it("decodes a fleet key when only the id string is in hand", () => {
    expect(resolveSessionRoute(sessionKey({ serverId: "srv-a", sessionId: "s1" }))).toEqual({
      ok: true,
      serverId: "srv-a",
      upstreamSessionId: "s1",
      source: "decode",
    });
  });

  it("agrees with the row path when the id string encodes the same session", () => {
    const consistent = row();
    expect(resolveSessionRoute(consistent.id)).toMatchObject({
      ok: true,
      serverId: consistent.serverId,
      upstreamSessionId: consistent.upstreamSessionId,
    });
  });

  it("reports unresolved for a bare id instead of throwing", () => {
    expect(() => resolveSessionRoute("s1")).not.toThrow();
    expect(resolveSessionRoute("s1")).toEqual({ ok: false, reason: "unresolved" });
  });

  it("reports unresolved for an empty id", () => {
    expect(resolveSessionRoute("")).toEqual({ ok: false, reason: "unresolved" });
  });
});
