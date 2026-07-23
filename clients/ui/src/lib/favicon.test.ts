import { describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
  FALLBACK_COLORS,
  PRIORITY,
  buildFaviconSvg,
  selectTopStatus,
  svgToDataUri,
} from "./favicon";

function mkSession(id: string, status?: string): SessionInfo {
  return {
    id,
    project: "p",
    command: "cmd",
    created_at: "2026-06-27T00:00:00Z",
    view: { card: { title: id }, status },
  };
}

describe("selectTopStatus — priority running > pending > waiting > idle > stopped > unknown", () => {
  it("returns unknown for an empty session list", () => {
    expect(selectTopStatus([])).toBe("unknown");
  });

  it("picks running over every other status", () => {
    expect(
      selectTopStatus([
        mkSession("a", "stopped"),
        mkSession("b", "idle"),
        mkSession("c", "running"),
        mkSession("d", "waiting"),
      ]),
    ).toBe("running");
  });

  it("picks pending when no running session exists", () => {
    expect(
      selectTopStatus([
        mkSession("a", "stopped"),
        mkSession("b", "pending"),
        mkSession("c", "waiting"),
      ]),
    ).toBe("pending");
  });

  it("picks waiting over idle and stopped", () => {
    expect(
      selectTopStatus([
        mkSession("a", "stopped"),
        mkSession("b", "idle"),
        mkSession("c", "waiting"),
      ]),
    ).toBe("waiting");
  });

  it("picks idle over stopped", () => {
    expect(selectTopStatus([mkSession("a", "stopped"), mkSession("b", "idle")])).toBe("idle");
  });

  it("falls back to stopped when only stopped sessions exist", () => {
    expect(selectTopStatus([mkSession("a", "stopped"), mkSession("b", "stopped")])).toBe("stopped");
  });

  it("normalizes unrecognised status strings to unknown", () => {
    expect(selectTopStatus([mkSession("a", "weird")])).toBe("unknown");
  });

  it("handles a missing status field as unknown", () => {
    expect(selectTopStatus([mkSession("a", undefined)])).toBe("unknown");
  });
});

describe("buildFaviconSvg — static shape + color encoding", () => {
  it("emits a 24×24 SVG root element", () => {
    const svg = buildFaviconSvg("running", FALLBACK_COLORS);
    expect(svg).toContain('viewBox="0 0 24 24"');
    expect(svg).toContain('xmlns="http://www.w3.org/2000/svg"');
  });

  it("burns the running color into both the ring and the arc", () => {
    const svg = buildFaviconSvg("running", FALLBACK_COLORS);
    expect(svg).toContain(FALLBACK_COLORS.running);
    expect(svg).toContain("<circle"); // faded ring
    expect(svg).toContain("<path"); // 3/4 arc
  });

  it("emits three dots for waiting", () => {
    const svg = buildFaviconSvg("waiting", FALLBACK_COLORS);
    expect(svg.match(/<circle/g)?.length).toBe(3);
  });

  it("emits a dashed circle for pending", () => {
    const svg = buildFaviconSvg("pending", FALLBACK_COLORS);
    expect(svg).toContain('stroke-dasharray="3 3"');
  });

  it("emits a rounded square for stopped", () => {
    const svg = buildFaviconSvg("stopped", FALLBACK_COLORS);
    expect(svg).toContain("<rect");
    expect(svg).toContain(FALLBACK_COLORS.stopped);
  });

  it("emits a horizontal dash for unknown", () => {
    const svg = buildFaviconSvg("unknown", FALLBACK_COLORS);
    expect(svg).toContain("<line");
  });

  it("emits a filled dot for idle", () => {
    const svg = buildFaviconSvg("idle", FALLBACK_COLORS);
    expect(svg).toContain("<circle");
    expect(svg).toContain(FALLBACK_COLORS.idle);
  });

  it("contains no <animate> or animation-related markup (favicon is static)", () => {
    for (const kind of PRIORITY) {
      const svg = buildFaviconSvg(kind, FALLBACK_COLORS);
      expect(svg).not.toMatch(/<animate/);
      expect(svg).not.toMatch(/keyframes/i);
    }
  });
});

describe("svgToDataUri", () => {
  it("prefixes with data:image/svg+xml, and percent-encodes the body", () => {
    const uri = svgToDataUri("<svg/>");
    expect(uri.startsWith("data:image/svg+xml,")).toBe(true);
    expect(uri).toContain("%3Csvg%2F%3E");
  });
});
