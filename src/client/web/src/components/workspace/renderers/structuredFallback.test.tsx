import { act, render, screen, waitFor } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { JsonTreeRenderer } from "./JsonTreeRenderer";
import { MermaidRenderer } from "./MermaidRenderer";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn().mockResolvedValue({ svg: "<svg>mermaid</svg>" }),
  },
}));

describe("verify-structured-fallback-bound", () => {
  afterEach(() => {
    vi.useRealTimers();
    vi.restoreAllMocks();
  });

  it("Mermaid: clean input renders structured within 300ms", async () => {
    const start = performance.now();
    render(<MermaidRenderer source={"graph TD; A-->B;"} />);
    await waitFor(() => expect(screen.getByTestId("mermaid-renderer")).toBeTruthy());
    expect(performance.now() - start).toBeLessThanOrEqual(300);
  });

  it("Mermaid: adversarial input renders fallback within 400ms", async () => {
    const mermaid = await import("mermaid");
    vi.mocked(mermaid.default.render).mockRejectedValueOnce(new Error("parse"));
    const start = performance.now();
    render(<MermaidRenderer source={"not valid {{"} />);
    await waitFor(() => expect(screen.getByText(/parse failed/i)).toBeTruthy());
    expect(performance.now() - start).toBeLessThanOrEqual(400);
    expect(screen.getByText(/not valid/)).toBeTruthy();
  });

  it("Mermaid: timeout renders fallback within 400ms", async () => {
    vi.useFakeTimers();
    const mermaid = await import("mermaid");
    vi.mocked(mermaid.default.render).mockImplementation(() => new Promise(() => {}));
    render(<MermaidRenderer source={"graph TD; A-->B;"} />);
    await act(async () => {
      vi.advanceTimersByTime(400);
      await Promise.resolve();
    });
    expect(screen.getByText(/timed out/i)).toBeTruthy();
    expect(screen.getByText(/graph TD/)).toBeTruthy();
  });

  it("JSON: clean input renders structured within 300ms", async () => {
    const start = performance.now();
    render(<JsonTreeRenderer source={'{"a":1,"b":[2]}'} />);
    await waitFor(() => expect(screen.getByTestId("json-tree-renderer")).toBeTruthy());
    expect(performance.now() - start).toBeLessThanOrEqual(300);
  });

  it("JSON: invalid input renders fallback within 400ms", async () => {
    const start = performance.now();
    render(<JsonTreeRenderer source="{not-json}" />);
    await waitFor(() => expect(screen.getByText(/Invalid JSON/i)).toBeTruthy());
    expect(performance.now() - start).toBeLessThanOrEqual(400);
    expect(screen.getByText(/{not-json}/)).toBeTruthy();
  });

  it("JSON: no blank loading pane past 400ms", async () => {
    const big = JSON.stringify(
      Object.fromEntries(Array.from({ length: 200 }, (_, i) => [`k${i}`, i])),
    );
    render(<JsonTreeRenderer source={big} />);
    await waitFor(
      () => {
        expect(screen.queryByText(/Parsing JSON/i)).toBeNull();
        expect(screen.getByTestId("json-tree-renderer")).toBeTruthy();
      },
      { timeout: 400 },
    );
  });
});
