import { beforeEach, describe, expect, it } from "vitest";
import { useThemeStore } from "./theme";
import type { Theme } from "./theme";

describe("useThemeStore", () => {
  beforeEach(() => {
    useThemeStore.setState({ theme: "system" });
  });

  it("initial theme is 'system'", () => {
    expect(useThemeStore.getState().theme).toBe("system");
  });

  it("setTheme('light') transitions to light", () => {
    useThemeStore.getState().setTheme("light");
    expect(useThemeStore.getState().theme).toBe("light");
  });

  it("setTheme('dark') transitions to dark", () => {
    useThemeStore.getState().setTheme("dark");
    expect(useThemeStore.getState().theme).toBe("dark");
  });

  it("setTheme('system') returns to system", () => {
    useThemeStore.getState().setTheme("dark");
    useThemeStore.getState().setTheme("system");
    expect(useThemeStore.getState().theme).toBe("system");
  });

  it("setTheme with invalid value is a no-op", () => {
    useThemeStore.getState().setTheme("light");
    useThemeStore.getState().setTheme("invalid" as Theme);
    expect(useThemeStore.getState().theme).toBe("light");
  });
});
