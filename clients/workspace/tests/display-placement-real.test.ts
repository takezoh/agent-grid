import { execFileSync } from "node:child_process";
import { createRequire } from "node:module";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";
import { listDisplayChoices, type DisplayScreenLike } from "../src/main/display-placement.js";

const enabled = process.platform === "win32" && process.env.AG_E2E_WORKSPACE_DISPLAY === "1";

describe.skipIf(!enabled)("FakeVsReal workspace display projection", () => {
  it("keeps the fakeable display contract aligned with Electron screen", () => {
    const require = createRequire(import.meta.url);
    const electronPath = require("electron") as string;
    const probePath = fileURLToPath(new URL("../scripts/display-probe.cjs", import.meta.url));
    const raw = execFileSync(electronPath, [probePath], {
      encoding: "utf8",
      windowsHide: true,
      timeout: 15_000,
    });
    const snapshot = JSON.parse(raw) as {
      primaryId: number;
      displays: Array<{
        id: number;
        label: string;
        bounds: { x: number; y: number; width: number; height: number };
        workArea: { x: number; y: number; width: number; height: number };
        dip: { x: number; y: number };
        physical: { x: number; y: number };
      }>;
    };
    expect(snapshot.displays.length).toBeGreaterThan(0);
    const primary = snapshot.displays.find((display) => display.id === snapshot.primaryId)!;
    const screen: DisplayScreenLike = {
      getAllDisplays: () => snapshot.displays,
      getPrimaryDisplay: () => primary,
      getDisplayMatching: () => primary,
      dipToScreenPoint: (point) => {
        const display = snapshot.displays.find(
          (candidate) => candidate.dip.x === point.x && candidate.dip.y === point.y,
        );
        if (!display) throw new Error("probe conversion point is missing");
        return display.physical;
      },
    };

    const choices = listDisplayChoices(screen);
    expect(choices).toHaveLength(snapshot.displays.length);
    expect(choices.find((choice) => choice.primary)?.id).toBe(String(snapshot.primaryId));
    expect(choices.map((choice) => choice.anchor)).toEqual(
      snapshot.displays.map((display) => display.physical),
    );
  });
});
