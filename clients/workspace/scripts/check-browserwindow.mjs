#!/usr/bin/env node
/**
 * contract-migration-window-per-session-invariant:
 * `new BrowserWindow(` may only appear in electron-window-factory.ts
 * (the sole production creation site owned by window-registry).
 *
 * Comments and string docs are ignored (word `BrowserWindow` alone is fine).
 */
import { readdirSync, readFileSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { fileURLToPath } from "node:url";

const root = join(fileURLToPath(new URL("..", import.meta.url)), "src");
const allowed = new Set(["main/electron-window-factory.ts"]);

// Require constructor call form so comments like `new BrowserWindow` in docs pass.
const re = /\bnew\s+BrowserWindow\s*\(/;
const offenders = [];

function stripComments(src) {
  // Rough strip: // line comments and /* block */ — good enough for this gate.
  return src
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/(^|[^:])\/\/.*$/gm, "$1");
}

function walk(dir) {
  for (const name of readdirSync(dir)) {
    const full = join(dir, name);
    const st = statSync(full);
    if (st.isDirectory()) {
      walk(full);
      continue;
    }
    if (!name.endsWith(".ts") && !name.endsWith(".js") && !name.endsWith(".mjs")) continue;
    const rel = relative(root, full).replaceAll("\\", "/");
    const text = stripComments(readFileSync(full, "utf8"));
    if (re.test(text) && !allowed.has(rel)) {
      offenders.push(rel);
    }
  }
}

walk(root);

if (offenders.length) {
  console.error("BrowserWindow created outside electron-window-factory.ts:");
  for (const o of offenders) console.error("  -", o);
  process.exit(1);
}
console.log("ok: BrowserWindow creation sites within allowlist");
