// driverShortcuts.test.ts — byte 定数 round-trip + lookup table の sanity.
//
// 視覚ラベルや aria は localised なので変わりやすいが、bytes は wire そのもの
// なので contract として固定する.

import { describe, expect, it } from "vitest";
import {
  BYTES_CTRL_C,
  BYTES_ESC,
  BYTES_SHIFT_TAB,
  DRIVER_SHORTCUTS,
  getDriverShortcuts,
} from "./driverShortcuts";

describe("driverShortcuts — byte constants", () => {
  it("Shift+Tab = CSI Z = \\x1b[Z (xterm kcbt)", () => {
    expect(BYTES_SHIFT_TAB).toBe("\x1b[Z");
    // 文字列長 3 = ESC + '[' + 'Z'.
    expect(BYTES_SHIFT_TAB.length).toBe(3);
  });

  it("Esc = \\x1b", () => {
    expect(BYTES_ESC).toBe("\x1b");
    expect(BYTES_ESC.length).toBe(1);
  });

  it("Ctrl-C = \\x03", () => {
    expect(BYTES_CTRL_C).toBe("\x03");
    expect(BYTES_CTRL_C.length).toBe(1);
  });
});

describe("driverShortcuts — DRIVER_SHORTCUTS table", () => {
  it("claude / codex に Mode / Esc / Ctrl-C を 3 つ持つ", () => {
    for (const key of ["claude", "codex"] as const) {
      const list = DRIVER_SHORTCUTS[key];
      expect(list).toBeDefined();
      expect(list?.length).toBe(3);
      const ids = list?.map((s) => s.id);
      expect(ids).toEqual(["mode", "esc", "ctrlc"]);
    }
  });

  it("claude.mode / codex.mode の bytes は Shift+Tab", () => {
    expect(DRIVER_SHORTCUTS.claude?.[0]?.bytes).toBe(BYTES_SHIFT_TAB);
    expect(DRIVER_SHORTCUTS.codex?.[0]?.bytes).toBe(BYTES_SHIFT_TAB);
  });
});

describe("getDriverShortcuts — lookup", () => {
  it("claude / codex は table の entry を返す", () => {
    expect(getDriverShortcuts("claude")).toBe(DRIVER_SHORTCUTS.claude);
    expect(getDriverShortcuts("codex")).toBe(DRIVER_SHORTCUTS.codex);
  });

  it.each(["shell", "gemini", "generic", "unknown-driver"])(
    "未対応 driver '%s' は [] を返す",
    (driver) => {
      expect(getDriverShortcuts(driver)).toEqual([]);
    },
  );

  it("null / undefined は [] を返す", () => {
    expect(getDriverShortcuts(null)).toEqual([]);
    expect(getDriverShortcuts(undefined)).toEqual([]);
    expect(getDriverShortcuts("")).toEqual([]);
  });
});
