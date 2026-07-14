import { readFileSync } from "node:fs";
import { dirname, join } from "node:path";
import { fileURLToPath } from "node:url";
import { describe, expect, it } from "vitest";

const webSrc = join(dirname(fileURLToPath(import.meta.url)), "../..");

describe("save allowlist", () => {
  it("asserts exactly one save() call site in FileViewer", () => {
    const fileViewer = readFileSync(join(webSrc, "components/workspace/FileViewer.tsx"), "utf8");
    const saveCalls = fileViewer.match(/api\.save\s*\(/g) ?? [];
    expect(saveCalls).toHaveLength(1);
  });

  it("WorkspaceDrawer does not call save directly", () => {
    const drawer = readFileSync(join(webSrc, "components/workspace/WorkspaceDrawer.tsx"), "utf8");
    expect(drawer.includes("api.save(")).toBe(false);
    expect(drawer.includes(".save(")).toBe(false);
  });
});
