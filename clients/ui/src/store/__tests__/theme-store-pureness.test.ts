import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

describe("theme.ts store pureness", () => {
  const themeStorePath = path.resolve(__dirname, "../theme.ts");
  const source = fs.readFileSync(themeStorePath, "utf-8");

  it("does not reference document", () => {
    expect(source).not.toMatch(/\bdocument\b/);
  });

  it("does not reference window", () => {
    expect(source).not.toMatch(/\bwindow\b/);
  });

  it("does not reference localStorage", () => {
    expect(source).not.toMatch(/\blocalStorage\b/);
  });

  it("does not reference matchMedia", () => {
    expect(source).not.toMatch(/\bmatchMedia\b/);
  });
});
