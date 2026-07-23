import { describe, expect, it } from "vitest";
import { loadThemeTokenMaps, parseDeclarations, resolveRaw } from "./cssTokenResolver";

describe("cssTokenResolver", () => {
  it("parses declarations from a block body", () => {
    const map = parseDeclarations(`
      --fg: #111;
      --bg: var(--color-neutral-250);
    `);
    expect(map.get("--fg")).toBe("#111");
    expect(map.get("--bg")).toBe("var(--color-neutral-250)");
  });

  it("resolves var() chains", () => {
    const tokens = parseDeclarations(`
      --color-neutral-250: #1e1e1e;
      --bg: var(--color-neutral-250);
      --fg: var(--color-neutral-950);
      --color-neutral-950: #e6e6e6;
    `);
    expect(resolveRaw(tokens, "--bg")).toBe("#1e1e1e");
    expect(resolveRaw(tokens, "--fg")).toBe("#e6e6e6");
  });

  it("merges light overrides onto :root primitives", () => {
    const css = `
      :root {
        --color-neutral-250: #1e1e1e;
        --color-neutral-950: #e6e6e6;
        --fg: var(--color-neutral-950);
        --bg: var(--color-neutral-250);
      }
      [data-theme="light"] { --color-neutral-250: #f5f5f5; --color-neutral-950: #1a1a1a; }
    `;
    const { dark, light } = loadThemeTokenMaps(css);
    expect(resolveRaw(dark, "--fg")).toBe("#e6e6e6");
    expect(resolveRaw(light, "--fg")).toBe("#1a1a1a");
    expect(resolveRaw(light, "--bg")).toBe("#f5f5f5");
  });
});
