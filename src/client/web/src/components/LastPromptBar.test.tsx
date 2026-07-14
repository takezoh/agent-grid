// LastPromptBar.test.tsx — per-driver visibility whitelist / prompt text
// rendering / empty placeholder / data-driver attribute.

import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { LastPromptBar } from "./LastPromptBar";

describe("LastPromptBar — per-driver visibility", () => {
  it.each(["claude", "codex", "gemini", "shell"])("renders the bar for %s", (driver) => {
    render(<LastPromptBar driver={driver} prompt="do the thing" />);
    const bar = screen.getByLabelText("Last user prompt");
    expect(bar.getAttribute("data-driver")).toBe(driver);
    expect(screen.getByText("do the thing")).toBeTruthy();
  });

  it("renders nothing for grok", () => {
    const { container } = render(<LastPromptBar driver="grok" prompt="do the thing" />);
    expect(container.firstChild).toBeNull();
  });

  it("renders nothing for generic sessions (root_driver = command first token)", () => {
    for (const driver of ["bash", "python", "vim", ""]) {
      const { container } = render(<LastPromptBar driver={driver} prompt="do the thing" />);
      expect(container.firstChild).toBeNull();
    }
  });

  it("renders nothing when driver is null / undefined", () => {
    const { container: c1 } = render(<LastPromptBar driver={null} prompt="p" />);
    expect(c1.firstChild).toBeNull();
    const { container: c2 } = render(<LastPromptBar driver={undefined} prompt="p" />);
    expect(c2.firstChild).toBeNull();
  });
});

describe("LastPromptBar — content", () => {
  it("shows a placeholder while the prompt is empty (area stays reserved)", () => {
    render(<LastPromptBar driver="claude" prompt={undefined} />);
    expect(screen.getByLabelText("Last user prompt")).toBeTruthy();
    expect(screen.getByText("No prompt yet")).toBeTruthy();
  });

  it("treats whitespace-only prompts as empty", () => {
    render(<LastPromptBar driver="codex" prompt={"  \n  "} />);
    expect(screen.getByText("No prompt yet")).toBeTruthy();
  });

  it("keeps multi-line prompt text in one clamped text node", () => {
    render(<LastPromptBar driver="claude" prompt={"line1\nline2\nline3\nline4"} />);
    const text = screen.getByText((_, el) => el?.className === "last-prompt-bar__text");
    expect(text.textContent).toBe("line1\nline2\nline3\nline4");
  });
});
