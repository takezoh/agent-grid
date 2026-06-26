// ADR-0049 (palette-ui-english-only) mechanical regression gate.
//
// Scans every .ts / .tsx under src/client/web/src/ and fails when a line
// contains hiragana, katakana, or CJK ideograph code points outside of a
// per-file allowlist. Catches accidental reintroduction of Japanese UI
// strings into the palette / web client source tree.
//
// Allowlist policy (per docs/specs/2026-06-24-web-ui-fixes/):
//   - This file itself is fully allowed. The regex it carries is written
//     with \u-escapes so it currently contains no Japanese code points,
//     but a future maintainer might add literal characters and we do not
//     want the gate to flag itself.
//   - Files that pre-date the english-only ADR and live outside the
//     palette scope (transcripts wire / global hotkey hook / daemon
//     store / etc.) are listed with regex `/./` (allow every line) until
//     they are migrated by a follow-up PR. Each entry is a single source
//     of truth so reviewers can audit and tighten the list in one place.
//
// Out of scope (per spec): Biome custom rule, anything outside src/, and
// extended Chinese / Korean punctuation ranges. Comments, string literals,
// and JSX text are not distinguished — the scan is purely line-based —
// so files that intentionally keep Japanese comments must be allowlisted.
import { readFileSync, readdirSync, statSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

// ESM-safe __dirname replacement; `module: ESNext` doesn't expose __dirname.
const HERE = dirname(fileURLToPath(import.meta.url));
const ROOT = resolve(HERE, "..");

// Hiragana + Katakana (U+3040-U+30FF) + CJK Unified Ideographs
// (U+4E00-U+9FFF). Written with \u escapes so this regex literal itself
// contains no Japanese characters — the file is its own negative test:
// if someone widens the range and accidentally matches this regex
// literal, the gate fails on no-japanese.test.ts and the maintainer
// notices immediately.
const JAPANESE = /[\u3040-\u30FF\u4E00-\u9FFF]/;

// Map of ROOT-relative path (POSIX-style forward slashes) → list of
// regexes applied per line. A line is considered allowed if it matches
// any of the regexes for its file. `/./` allows every line in the file.
//
// Keep this map sorted alphabetically for diff readability.
const ALLOWLIST: Record<string, RegExp[]> = {
  // Out-of-scope legacy comments / test fixtures. These files are not
  // the palette UI surface that ADR-0049 governs; they are tracked
  // here so the gate stays green and any *new* Japanese in palette
  // code is caught. Tighten in follow-up PRs.
  //
  // The 6 in-scope palette / hotkey / app shell files were translated
  // to English as part of the integration cleanup (post-review). Their
  // entries have been removed from this allowlist so the gate now
  // catches any future Japanese reintroduction in those files for real.
  // App.tsx carries the confirm-dialog copy for the session-terminate feature
  // (Japanese UI strings: title / body / button labels). Tracked here so the
  // gate stays green; future palette additions to App.tsx still need to be
  // English-only — only the confirm-dialog wiring is in scope of this allow.
  "App.tsx": [/./],
  "App.test.tsx": [/./],
  "api/transcripts.test.ts": [/./],
  "api/transcripts.ts": [/./],
  // Mobile terminal UX surface (web-terminal-mobile-ux spec, ADR 0067-0075).
  // ADR-0049 governs the *palette* UI; the mobile experience is a separate
  // surface whose aria-labels / announcements are spec-mandated Japanese
  // ("キーボードを開く", "閲覧モードに戻りました", "最新へ移動できます", "文字サイズ",
  // etc. — see ux.md / spec.md). They are tracked here as out-of-palette-scope,
  // mirroring the transcripts / daemon legacy entries. New mobile work that does
  // NOT need a spec string should still prefer \u escapes (cf. TerminalPane
  // baseline / chunk-07 integration files, which carry no Japanese at all).
  "components/AriaLiveStatus.test.tsx": [/./],
  "components/AriaLiveStatus.tsx": [/./],
  // Session terminate (PC + mobile UX) + driver shortcut bar (mobile UX) ship
  // Japanese aria-labels and confirm-dialog copy (driver mode 切替 / Ctrl-C
  // 中断 / セッション終了 / キャンセル). Same precedent as KeyboardFAB et al.
  "components/ConfirmDialog.test.tsx": [/./],
  "components/ConfirmDialog.tsx": [/./],
  "components/DriverShortcutBar.test.tsx": [/./],
  "components/DriverShortcutBar.tsx": [/./],
  "components/FontSizeControl.test.tsx": [/./],
  "components/FontSizeControl.tsx": [/./],
  "components/JumpToLatestFAB.test.tsx": [/./],
  "components/JumpToLatestFAB.tsx": [/./],
  "components/KeyboardFAB.test.tsx": [/./],
  "components/KeyboardFAB.tsx": [/./],
  "components/SessionList.tsx": [/./],
  "components/SessionTerminateButton.test.tsx": [/./],
  "components/SessionTerminateButton.tsx": [/./],
  "components/TerminalMobileOverlay.tsx": [/./],
  "components/palette/ParamEmptyState.test.tsx": [/./],
  "components/primitives/IconButton.test.tsx": [/./],
  "hooks/useAnnouncer.test.ts": [/./],
  "hooks/useHostPointerInterceptor.test.ts": [/./],
  "hooks/useHostPointerInterceptor.ts": [/./],
  "hooks/useInputMode.test.ts": [/./],
  "hooks/useInputMode.ts": [/./],
  "hooks/useJumpToLatest.ts": [/./],
  "hooks/useTerminateSession.test.ts": [/./],
  "hooks/useTerminateSession.ts": [/./],
  "hooks/useTranscript.ts": [/./],
  "lib/driverShortcuts.test.ts": [/./],
  "lib/driverShortcuts.ts": [/./],
  "store/daemon.test.ts": [/./],
  "store/daemon.ts": [/./],
  "wire/codec.ts": [/./],
  "wire/server.ts": [/./],
  // The gate itself — see header comment.
  "__meta__/no-japanese.test.ts": [/./],
};

/** Recursively collect ROOT-relative .ts / .tsx paths under `dir`. */
function collectSources(dir: string, rootRel = ""): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    const abs = `${dir}/${entry}`;
    const rel = rootRel === "" ? entry : `${rootRel}/${entry}`;
    const st = statSync(abs);
    if (st.isDirectory()) {
      out.push(...collectSources(abs, rel));
      continue;
    }
    if (entry.endsWith(".ts") || entry.endsWith(".tsx")) {
      out.push(rel);
    }
  }
  return out;
}

describe("no Japanese in web client source", () => {
  const files = collectSources(ROOT).sort();
  for (const rel of files) {
    it(`${rel} contains no Japanese`, () => {
      const content = readFileSync(resolve(ROOT, rel), "utf8");
      const lines = content.split("\n");
      const allow = ALLOWLIST[rel] ?? [];
      const violations: string[] = [];
      lines.forEach((line: string, i: number) => {
        if (!JAPANESE.test(line)) return;
        if (allow.some((rx) => rx.test(line))) return;
        violations.push(`${rel}:${i + 1}: ${line.trim()}`);
      });
      expect(violations).toEqual([]);
    });
  }
});
