import { describe, expect, it } from "vitest";
import type { SessionInfo, SessionStatus } from "../wire/server";
import {
  FALLBACK_COLORS,
  PRIORITY,
  buildFaviconSvg,
  selectTopStatus,
  svgToDataUri,
} from "./favicon";

function mkSession(id: string, status: SessionStatus): SessionInfo {
  return {
    id,
    project: "p",
    command: "cmd",
    created_at: "2026-06-27T00:00:00Z",
    status,
    view: { card: { title: id } },
  };
}

const composite = (
  lifecycle: SessionStatus["lifecycle"],
  human_input = false,
  failure: SessionStatus["failure"] = "none",
): SessionStatus => ({ lifecycle, human_input, failure });

describe("selectTopStatus", () => {
  it("uses failed > action > running > idle > stopped", () => {
    expect(selectTopStatus([])).toBe("unknown");
    expect(selectTopStatus([mkSession("s", composite("stopped"))])).toBe("stopped");
    expect(
      selectTopStatus([
        mkSession("s", composite("stopped")),
        mkSession("i", composite("idle")),
        mkSession("r", composite("active")),
      ]),
    ).toBe("running");
    expect(
      selectTopStatus([
        mkSession("r", composite("active")),
        mkSession("a", composite("active", true)),
      ]),
    ).toBe("action");
    expect(
      selectTopStatus([
        mkSession("a", composite("active", true)),
        mkSession("f", composite("idle", false, "failed")),
      ]),
    ).toBe("failed");
  });
});

describe("buildFaviconSvg", () => {
  it.each(PRIORITY)("renders static geometry for %s", (kind) => {
    const svg = buildFaviconSvg(kind, FALLBACK_COLORS);
    expect(svg).toContain('viewBox="0 0 24 24"');
    expect(svg).not.toMatch(/<animate|keyframes/i);
  });

  it("renders ! for action and x for failure", () => {
    expect(buildFaviconSvg("action", FALLBACK_COLORS)).toContain(">!</text>");
    expect(buildFaviconSvg("failed", FALLBACK_COLORS)).toContain("<path");
  });
});

describe("svgToDataUri", () => {
  it("percent-encodes the SVG", () => {
    expect(svgToDataUri("<svg/>")).toContain("data:image/svg+xml,%3Csvg%2F%3E");
  });
});
