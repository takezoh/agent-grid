import * as fs from "node:fs/promises";
import * as path from "node:path";
import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { NewSessionButton } from "./NewSessionButton";
import { SidebarBrandRow } from "./SidebarBrandRow";

describe("m2 shell structure (FR-008 / FR-014)", () => {
  it("SidebarBrandRow renders product name and command trigger", () => {
    render(<SidebarBrandRow />);
    expect(screen.getByText("agent-grid")).toBeTruthy();
    expect(screen.getByLabelText("Open command menu")).toBeTruthy();
    expect(screen.getByRole("button", { name: "Open command menu" }).dataset.role).toBe(
      "command-search-trigger",
    );
  });

  it("NewSessionButton renders bottom-anchored CTA", () => {
    render(<NewSessionButton />);
    const btn = screen.getByRole("button", { name: "New session" });
    expect(btn.dataset.role).toBe("new-session-button");
  });

  it("shell.css places hamburger before header content (left edge)", async () => {
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "shell.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/\.hamburger-toggle\s*\{[\s\S]*?order:\s*-1/);
  });

  it("session-list.css uses --accent-soft for selected rows (FR-010)", async () => {
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "session-list.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toContain("var(--accent-soft)");
    expect(source).toContain("var(--focus-ring)");
  });
});
