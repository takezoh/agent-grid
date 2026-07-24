import { describe, expect, it } from "vitest";
import { shouldOpenHome } from "../src/main/app-entry.js";

describe("Workspace startup mode", () => {
  it("opens the standalone home UI for a direct launch", () => {
    expect(shouldOpenHome([])).toBe(true);
    expect(shouldOpenHome(["--config-dir", "C:\\config"])).toBe(true);
  });

  it("stays in control-endpoint mode when launched by Shell", () => {
    expect(shouldOpenHome(["--config-dir", "C:\\config", "--background"])).toBe(false);
  });
});
