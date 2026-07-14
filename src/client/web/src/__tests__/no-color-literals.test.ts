/**
 * no-color-literals.test.ts
 *
 * FR-006: CSS files under src/css/ (except tokens.css) must not contain
 * hex/rgb/rgba/hsl color literals. All colors resolve via semantic tokens.
 */

import * as fs from "node:fs";
import * as path from "node:path";
import { describe, expect, it } from "vitest";

const cssDir = path.resolve(__dirname, "../css");

const HEX_PATTERN = /#[0-9a-fA-F]{3,8}\b/g;
const RGB_PATTERN = /\brgba?\([^)]+\)/g;
const HSL_PATTERN = /\bhsla?\([^)]+\)/g;

function stripComments(content: string): string {
  return content.replace(/\/\*[\s\S]*?\*\//g, "");
}

function findColorLiterals(content: string): string[] {
  const stripped = stripComments(content);
  const matches: string[] = [];
  for (const pattern of [HEX_PATTERN, RGB_PATTERN, HSL_PATTERN]) {
    const found = stripped.match(pattern) ?? [];
    matches.push(...found);
  }
  return matches;
}

describe("FR-006: no color literals outside tokens.css", () => {
  const cssFiles = fs
    .readdirSync(cssDir)
    .filter((f) => f.endsWith(".css") && f !== "tokens.css")
    .sort();

  for (const filename of cssFiles) {
    it(`${filename} has no hex/rgb/rgba/hsl color literals`, () => {
      const content = fs.readFileSync(path.join(cssDir, filename), "utf-8");
      const literals = findColorLiterals(content);
      expect(literals, `Found color literals in ${filename}: ${literals.join(", ")}`).toHaveLength(
        0,
      );
    });
  }
});
