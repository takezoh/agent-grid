// Ownership gate for the session-identity and wire-frame boundaries
// (adr-fleet-session-identity-boundary-ownership).
//
// The design closes these boundaries with API shape and type splits, which the
// compiler enforces on its own. This gate covers the one thing shape cannot:
// a new module importing the identity helpers or the frame serializer directly
// and quietly becoming a thirteenth crossing. Each import is legal TypeScript;
// what makes it wrong is who is doing it.
//
// Owners:
//   sessionKey / parseSessionKey  → fleet/session-identity.ts (defines them),
//                                   socket/connection.ts (the wire boundary)
//   serializeClientFrame          → socket/connection.ts (assembles frames)
//   selectCodec / generated stub  → wire/codec.ts (the codec seam)
//
// What this gate does NOT catch, stated so nobody reads a green run as more
// than it is:
//   - Implicit dependencies that never import a helper. Both crossings the
//     original horizontal scan missed (the viewUpdate activity_session_id and
//     the per-event session_id) were exactly this shape: they handled a session
//     id without ever naming sessionKey or parseSessionKey. Only reading the
//     data flow finds those.
//   - The test tree. tsconfig excludes *.test.ts(x) from the typecheck, and
//     this scan skips them too, so a test may construct ids however it likes.
//   - clients/workspace. It is a separate package with its own tsconfig; the
//     second fleet-key encoder in src/shared/session-ref.ts is tracked by a
//     follow-up issue, not by this scan.
//   - Runtime behavior of any kind. This is a source-text scan; it proves who
//     imports what, never what the code then does.

import { readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, join, relative, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const HERE = dirname(fileURLToPath(import.meta.url));
const SRC = resolve(HERE, "..");

/** Production source only: the scan skips tests and meta trees. */
function productionSources(dir: string, out: string[] = []): string[] {
  for (const entry of readdirSync(dir)) {
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) {
      if (entry === "__meta__" || entry === "__tests__") continue;
      productionSources(full, out);
      continue;
    }
    if (!/\.tsx?$/.test(entry)) continue;
    if (/\.test\.tsx?$/.test(entry)) continue;
    out.push(full);
  }
  return out;
}

function posix(path: string): string {
  return relative(SRC, path).split("\\").join("/");
}

/** Named imports pulled from any module specifier, per source line. */
function importedNames(source: string): Set<string> {
  const names = new Set<string>();
  for (const match of source.matchAll(/import\s+(type\s+)?\{([^}]*)\}\s*from\s*["'][^"']+["']/g)) {
    for (const raw of (match[2] ?? "").split(",")) {
      const name = raw
        .trim()
        .replace(/^type\s+/, "")
        .split(/\s+as\s+/)[0]
        ?.trim();
      if (name) names.add(name);
    }
  }
  return names;
}

type Rule = { symbols: string[]; owners: string[]; why: string };

const RULES: Rule[] = [
  {
    symbols: ["sessionKey", "parseSessionKey"],
    owners: ["fleet/session-identity.ts", "socket/connection.ts"],
    why:
      "Handling the fleet key syntactically outside its owners is how a new crossing " +
      "gets created. Route through resolveSessionRoute, or hold a FleetSessionRow.",
  },
  {
    symbols: ["serializeClientFrame"],
    owners: ["socket/connection.ts", "wire/codec.ts"],
    why:
      "Assembling and serializing a wire frame is Connection's job. retry.ts used to " +
      "do it, which is how a frame could reach the socket without passing the id owner.",
  },
  {
    symbols: ["selectCodec", "generatedCodecStub"],
    owners: ["wire/codec.ts"],
    why: "The codec seam has one entry point; importing the selector elsewhere forks it.",
  },
];

function violations(rule: Rule, files: string[]): string[] {
  const found: string[] = [];
  for (const file of files) {
    const rel = posix(file);
    if (rule.owners.includes(rel)) continue;
    const names = importedNames(readFileSync(file, "utf8"));
    for (const symbol of rule.symbols) {
      if (names.has(symbol)) found.push(`${rel} imports ${symbol}`);
    }
  }
  return found.sort();
}

describe("session identity and wire-frame ownership", () => {
  const files = productionSources(SRC);

  it("scans a non-trivial production tree", () => {
    // Guard against the scan silently covering nothing — a broken walk would
    // otherwise make every assertion below vacuously true.
    expect(files.length).toBeGreaterThan(50);
  });

  it.each(RULES)("keeps $symbols inside their owners", (rule) => {
    expect(violations(rule, files)).toEqual([]);
  });

  it("every declared owner exists", () => {
    const existing = new Set(files.map(posix));
    for (const rule of RULES) {
      for (const owner of rule.owners) {
        expect(existing.has(owner), `owner ${owner} is not a production source`).toBe(true);
      }
    }
  });

  // Negative test: the gate has to fail on a real violation, not merely pass on
  // a clean tree. A rule that matched nothing would look identical to a rule
  // that holds.
  it("fails when a non-owner imports an owned symbol", () => {
    const synthetic = join(SRC, "store", "synthetic-violation.ts");
    const scan = (source: string, rule: Rule) => {
      const names = importedNames(source);
      return rule.symbols.filter((s) => names.has(s));
    };

    expect(
      scan(`import { sessionKey } from "../fleet/session-identity";\n`, RULES[0] as Rule),
    ).toEqual(["sessionKey"]);
    expect(
      scan(`import { serializeClientFrame } from "../wire/codec";\n`, RULES[1] as Rule),
    ).toEqual(["serializeClientFrame"]);
    // Type-only imports count too: a type import is enough to start reasoning
    // about the id space outside the owner.
    expect(
      scan(`import type { parseSessionKey } from "../fleet/session-identity";\n`, RULES[0] as Rule),
    ).toEqual(["parseSessionKey"]);
    // And the synthetic path is not an owner, so the rule would apply to it.
    expect(RULES[0]?.owners).not.toContain(posix(synthetic));
  });
});
