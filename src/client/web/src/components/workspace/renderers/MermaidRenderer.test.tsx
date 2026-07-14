import { render, screen, waitFor } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { MermaidRenderer } from "./MermaidRenderer";

vi.mock("mermaid", () => ({
  default: {
    initialize: vi.fn(),
    render: vi.fn().mockResolvedValue({ svg: "<svg>mermaid</svg>" }),
  },
}));

describe("MermaidRenderer", () => {
  it("renders SVG for valid mermaid", async () => {
    render(<MermaidRenderer source={"graph TD; A-->B;"} />);
    await waitFor(() => expect(screen.getByTestId("mermaid-renderer")).toBeTruthy());
  });

  it("fallback on invalid mermaid", async () => {
    const mermaid = await import("mermaid");
    vi.mocked(mermaid.default.render).mockRejectedValueOnce(new Error("parse"));
    render(<MermaidRenderer source={"not valid {{"} />);
    await waitFor(() => expect(screen.getByText(/parse failed/i)).toBeTruthy());
  });
});
