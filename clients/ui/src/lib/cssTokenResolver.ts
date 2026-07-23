/**
 * cssTokenResolver — parse tokens.css blocks and resolve var() chains.
 * Used by token-contrast.test.ts (NFR-001 / m1 contrast verification).
 */

import * as fs from "node:fs";

export type TokenMap = Map<string, string>;

const VAR_REF = /^var\(\s*(--[\w-]+)(?:\s*,\s*([^)]+))?\s*\)$/;

/** Extract `--name: value;` declarations from a CSS block body. */
export function parseDeclarations(blockBody: string): TokenMap {
  const tokens = new Map<string, string>();
  const declRe = /(--[\w-]+)\s*:\s*([^;]+);/g;
  let match: RegExpExecArray | null = declRe.exec(blockBody);
  while (match !== null) {
    const name = match[1];
    const value = match[2];
    if (name && value) tokens.set(name, value.trim());
    match = declRe.exec(blockBody);
  }
  return tokens;
}

/** Pull the first `{ ... }` body for a selector substring in a CSS file. */
export function extractBlock(css: string, selector: string): string {
  const idx = css.indexOf(selector);
  if (idx === -1) return "";
  const open = css.indexOf("{", idx);
  if (open === -1) return "";
  let depth = 0;
  for (let i = open; i < css.length; i++) {
    if (css[i] === "{") depth++;
    else if (css[i] === "}") {
      depth--;
      if (depth === 0) return css.slice(open + 1, i);
    }
  }
  return "";
}

/** Load dark (:root) and light ([data-theme="light"]) token maps from tokens.css. */
export function loadThemeTokenMaps(tokensCss: string): { dark: TokenMap; light: TokenMap } {
  const rootBody = extractBlock(tokensCss, ":root");
  const lightBody = extractBlock(tokensCss, '[data-theme="light"]');
  const dark = parseDeclarations(rootBody);
  const lightBase = new Map(dark);
  for (const [k, v] of parseDeclarations(lightBody)) {
    lightBase.set(k, v);
  }
  return { dark, light: lightBase };
}

export function loadThemeTokenMapsFromFile(filePath: string): { dark: TokenMap; light: TokenMap } {
  return loadThemeTokenMaps(fs.readFileSync(filePath, "utf-8"));
}

/** Resolve a custom property to its literal CSS value (may still be var()). */
export function resolveRaw(tokens: TokenMap, name: string, depth = 0): string | null {
  if (depth > 32) return null;
  const raw = tokens.get(name);
  if (raw === undefined) return null;
  const trimmed = raw.trim();
  const m = VAR_REF.exec(trimmed);
  if (!m) return trimmed;
  const ref = m[1];
  if (!ref) return null;
  const resolved = resolveRaw(tokens, ref, depth + 1);
  if (resolved !== null) return resolved;
  return m[2]?.trim() ?? null;
}
