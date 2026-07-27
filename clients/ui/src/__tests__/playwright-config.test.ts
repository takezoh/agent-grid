import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { describe, expect, it } from "vitest";

function configSource(name: string): string {
  return readFileSync(resolve(process.cwd(), name), "utf8");
}

describe("Playwright harness ownership", () => {
  it("keeps real-stack specs out of the deterministic fake-backend gate", () => {
    expect(configSource("playwright.config.ts")).toMatch(/testIgnore:.*real-stack/);
  });

  it("scopes the fidelity config to real-stack specs", () => {
    expect(configSource("playwright.real.config.ts")).toContain('testDir: "./e2e/real-stack"');
  });

  it("leaves server lifecycle ownership to the explicit harness scripts", () => {
    expect(configSource("playwright.config.ts")).not.toContain("webServer:");
    expect(configSource("playwright.real.config.ts")).not.toContain("webServer:");
    expect(configSource("package.json")).toContain("./scripts/e2e-smoke.sh");
    expect(configSource("package.json")).toContain("./scripts/e2e-real-stack.sh");
  });
});
