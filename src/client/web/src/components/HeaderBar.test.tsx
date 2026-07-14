import { fireEvent, render, screen } from "@testing-library/react";
import { describe, expect, it, vi } from "vitest";
import { HeaderBar } from "./HeaderBar";

describe("HeaderBar (FR-011 / UAC-004)", () => {
  it("renders breadcrumb with project basename and session title", () => {
    render(
      <HeaderBar
        project="/home/dev/dev/agent-grid"
        card={{ title: "Fix WS reconnect backoff" }}
        status="running"
        model="gpt-5"
        effort="high"
        driver="claude"
      />,
    );
    expect(screen.getByText("agent-grid")).toBeTruthy();
    expect(screen.getByText("Fix WS reconnect backoff")).toBeTruthy();
    expect(screen.getByText("agent-grid / Fix WS reconnect backoff".split(" / ")[0])).toBeTruthy();
    expect(screen.getByLabelText("session metadata")?.textContent).toBe("claude · gpt-5 · high");
    expect(screen.getByLabelText(/status: running/)).toBeTruthy();
  });

  it("mobile layout shows session title only (FR-013)", () => {
    render(
      <HeaderBar
        mobile
        project="/home/dev/dev/agent-grid"
        card={{ title: "Mobile session" }}
        status="idle"
      />,
    );
    expect(screen.getByText("Mobile session")).toBeTruthy();
    expect(screen.queryByText("agent-grid")).toBeNull();
  });

  it("header bar has 44px height contract in shell.css", async () => {
    const fs = await import("node:fs/promises");
    const path = await import("node:path");
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "shell.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/\.header-bar\s*\{[\s\S]*?height:\s*44px/);
    expect(source).toMatch(/\.app-header-area\s*\{[\s\S]*?height:\s*44px/);
  });
});

describe("HeaderBar — terminate button (FR-024 / UAC-011)", () => {
  it("renders stop icon button when sessionId and onRequestTerminate are provided", () => {
    const { container } = render(
      <HeaderBar
        card={{ title: "alpha" }}
        status="running"
        sessionId="s1"
        onRequestTerminate={vi.fn()}
      />,
    );
    expect(container.querySelector(".session-terminate-button")).not.toBeNull();
    expect(container.querySelector(".run-state-badge")).not.toBeNull();
  });

  it("omits stop button when onRequestTerminate is absent", () => {
    const { container } = render(
      <HeaderBar card={{ title: "alpha" }} status="running" sessionId="s1" />,
    );
    expect(container.querySelector(".session-terminate-button")).toBeNull();
  });

  it("click fires onRequestTerminate(id, label, opener)", () => {
    const onRequest = vi.fn();
    render(
      <HeaderBar
        card={{ title: "alpha" }}
        status="running"
        sessionId="s-id-42"
        onRequestTerminate={onRequest}
      />,
    );
    const btn = screen.getByRole("button", { name: 'Stop "alpha"' });
    fireEvent.click(btn);
    expect(onRequest).toHaveBeenCalledTimes(1);
    expect(onRequest.mock.calls[0]?.[0]).toBe("s-id-42");
    expect(onRequest.mock.calls[0]?.[1]).toBe("alpha");
    expect(onRequest.mock.calls[0]?.[2]).toBe(btn);
  });

  it("uses New Session placeholder when card.title is absent", () => {
    const onRequest = vi.fn();
    render(
      <HeaderBar card={{}} status="running" sessionId="s-empty" onRequestTerminate={onRequest} />,
    );
    const btn = screen.getByRole("button", { name: 'Stop "New Session"' });
    fireEvent.click(btn);
    expect(onRequest.mock.calls[0]?.[1]).toBe("New Session");
  });

  it("stop button meets 36px minimum hit target in shell.css", async () => {
    const fs = await import("node:fs/promises");
    const path = await import("node:path");
    const cssPath = path.join(
      import.meta.dirname ?? __dirname,
      "..",
      "css",
      "session-terminate.css",
    );
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/min-width:\s*36px/);
    expect(source).toMatch(/min-height:\s*36px/);
  });
});

describe("HeaderBar — FR-022 dissolution (UAC-010)", () => {
  it("does not render a standalone driver view panel", () => {
    const { container } = render(
      <HeaderBar
        project="/home/dev/dev/agent-grid"
        card={{ title: "Session title", tags: [{ text: "tag-a" }] }}
        status="running"
        model="gpt-5"
        effort="high"
        driver="claude"
      />,
    );
    expect(container.querySelector(".driver-view-panel")).toBeNull();
    expect(container.querySelector('[aria-label="driver view"]')).toBeNull();
  });
});
