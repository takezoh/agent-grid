/**
 * Verifies the matchMedia mock and setMatchMedia helper installed by test-setup.ts.
 *
 * These tests run in happy-dom where window.matchMedia is not natively available.
 * The mock is loaded globally via the setupFiles config in vitest.config.ts.
 */
import { describe, expect, it, vi } from "vitest";

describe("matchMedia mock (test-setup)", () => {
  it("returns a MediaQueryList-like object for any query", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    expect(mql).toBeDefined();
    expect(typeof mql.matches).toBe("boolean");
    expect(mql.media).toBe("(prefers-color-scheme: dark)");
  });

  it("has default matches=true for prefers-color-scheme: dark", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    expect(mql.matches).toBe(true);
  });

  it("has default matches=false for prefers-reduced-motion: reduce", () => {
    const mql = matchMedia("(prefers-reduced-motion: reduce)");
    expect(mql.matches).toBe(false);
  });

  it("has default matches=false for unknown queries", () => {
    const mql = matchMedia("(unknown-feature: value)");
    expect(mql.matches).toBe(false);
  });
});

describe("setMatchMedia helper", () => {
  it("updates matches on an existing MediaQueryList", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    // default is true
    expect(mql.matches).toBe(true);

    globalThis.setMatchMedia("(prefers-color-scheme: dark)", false);

    // The same mql object reflects the updated value via the getter.
    expect(mql.matches).toBe(false);
  });

  it("fires change listeners with the updated matches value", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const listener = vi.fn();
    mql.addEventListener("change", listener);

    globalThis.setMatchMedia("(prefers-color-scheme: dark)", false);

    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith(
      expect.objectContaining({ matches: false, media: "(prefers-color-scheme: dark)" }),
    );
  });

  it("fires change listener with matches=true when set back to true", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const listener = vi.fn();
    mql.addEventListener("change", listener);

    globalThis.setMatchMedia("(prefers-color-scheme: dark)", false);
    globalThis.setMatchMedia("(prefers-color-scheme: dark)", true);

    expect(listener).toHaveBeenCalledTimes(2);
    expect(listener).toHaveBeenLastCalledWith(
      expect.objectContaining({ matches: true, media: "(prefers-color-scheme: dark)" }),
    );
  });

  it("does not call removed listeners", () => {
    const mql = matchMedia("(prefers-color-scheme: dark)");
    const listener = vi.fn();
    mql.addEventListener("change", listener);
    mql.removeEventListener("change", listener);

    globalThis.setMatchMedia("(prefers-color-scheme: dark)", false);

    expect(listener).not.toHaveBeenCalled();
  });

  it("works for prefers-reduced-motion query", () => {
    const mql = matchMedia("(prefers-reduced-motion: reduce)");
    const listener = vi.fn();
    mql.addEventListener("change", listener);

    globalThis.setMatchMedia("(prefers-reduced-motion: reduce)", true);

    expect(mql.matches).toBe(true);
    expect(listener).toHaveBeenCalledTimes(1);
    expect(listener).toHaveBeenCalledWith(
      expect.objectContaining({ matches: true, media: "(prefers-reduced-motion: reduce)" }),
    );
  });
});

describe("matchMedia mock afterEach reset", () => {
  it("previous test's setMatchMedia does not leak (dark scheme is back to default true)", () => {
    // If afterEach reset works, previous tests setting dark=false are gone.
    const mql = matchMedia("(prefers-color-scheme: dark)");
    expect(mql.matches).toBe(true);
  });

  it("previous test's setMatchMedia does not leak (reduced-motion is back to default false)", () => {
    const mql = matchMedia("(prefers-reduced-motion: reduce)");
    expect(mql.matches).toBe(false);
  });
});
