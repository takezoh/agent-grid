import { render, screen } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { DiffViewer } from "./DiffViewer";

describe("DiffViewer", () => {
  it("verify-diff-layout-a11y: changed lines have cues", () => {
    render(
      <DiffViewer
        diff={{
          outcome: "ok",
          diff: "+++ a\n--- b\n+added\n-removed\n~changed\n context\n",
        }}
      />,
    );
    const lines = screen.getAllByRole("listitem");
    expect(lines.length).toBeGreaterThan(0);
    expect(document.querySelector(".workspace-diff__cue")).toBeTruthy();
  });

  it("verify-diff-base-outcomes: distinct banner for not_a_repo", () => {
    render(<DiffViewer diff={{ outcome: "not_a_repo" }} />);
    expect(screen.getByText(/not a git repository/i)).toBeTruthy();
  });

  it("verify-diff-base-outcomes: distinct banner for git_metadata_corrupted", () => {
    render(<DiffViewer diff={{ outcome: "git_metadata_corrupted" }} />);
    expect(screen.getByText(/Git metadata in this workspace appears corrupted/i)).toBeTruthy();
  });

  it("verify-diff-base-outcomes: distinct banner for git_binary_missing", () => {
    render(<DiffViewer diff={{ outcome: "git_binary_missing" }} />);
    expect(screen.getByText(/git binary was not found/i)).toBeTruthy();
  });

  it("verify-diff-layout-a11y: large-diff folding visible for 5000-line case", () => {
    const changed = Array.from({ length: 5001 }, (_, i) => `+line${i}`).join("\n");
    render(<DiffViewer diff={{ outcome: "ok", diff: changed }} />);
    expect(screen.getByRole("button", { name: /hidden changed lines/i })).toBeTruthy();
  });

  it("verify-diff-layout-a11y: simulated scroll frame-time p95 <= 33ms", () => {
    const changed = Array.from({ length: 6000 }, (_, i) => `+line${i}`).join("\n");
    const { container } = render(<DiffViewer diff={{ outcome: "ok", diff: changed }} />);
    const scroller = container.querySelector(".workspace-diff__lines")?.parentElement;
    expect(scroller).toBeTruthy();
    const samples: number[] = [];
    for (let i = 0; i < 40; i++) {
      const start = performance.now();
      scroller?.dispatchEvent(new Event("scroll"));
      samples.push(performance.now() - start);
    }
    samples.sort((a, b) => a - b);
    const p95 = samples[Math.floor(samples.length * 0.95)] ?? 0;
    expect(p95).toBeLessThanOrEqual(33);
  });
});
