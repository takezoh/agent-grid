import { act, cleanup, render, screen } from "@testing-library/react";
import { afterEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { StatusBar } from "./StatusBar";

describe("StatusBar (FR-023 / UAC-010)", () => {
  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    useDaemonStore.getState().reset();
  });

  it("renders status_line on the left when present", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    render(<StatusBar statusLine="Running task" />);
    expect(screen.getByText("Running task")).toBeTruthy();
  });

  it("stays visible when status_line is absent and connection is nominal, showing 'connected'", () => {
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });
    const { container } = render(<StatusBar />);
    expect(container.querySelector("[data-role='status-bar']")).not.toBeNull();
    expect(screen.getByLabelText("connection state")?.textContent).toBe("connected");
  });

  it("shows connection state on the right when transport is degraded", () => {
    useDaemonStore.setState({ status: "reconnecting", daemonDisconnected: false });
    render(<StatusBar />);
    expect(screen.getByLabelText("connection state")?.textContent).toBe("reconnecting to server…");
  });

  it("shows both status_line and connection state when both apply", () => {
    useDaemonStore.setState({ status: "reconnecting", daemonDisconnected: false });
    render(<StatusBar statusLine="Syncing changes" />);
    expect(screen.getByText("Syncing changes")).toBeTruthy();
    expect(screen.getByLabelText("connection state")?.textContent).toBe("reconnecting to server…");
  });

  it("renders ticking elapsed next to status_line", () => {
    vi.useFakeTimers();
    vi.setSystemTime(new Date("2026-06-20T00:00:00Z"));
    useDaemonStore.setState({ status: "open", daemonDisconnected: false });

    render(<StatusBar statusLine="Running task" statusChangedAt="2026-06-19T23:59:55Z" />);
    expect(screen.getByLabelText("elapsed").textContent).toBe("5s");

    act(() => {
      vi.advanceTimersByTime(2000);
    });
    expect(screen.getByLabelText("elapsed").textContent).toBe("7s");
  });

  it("status-bar has 26px height contract in view.css", async () => {
    const fs = await import("node:fs/promises");
    const path = await import("node:path");
    const cssPath = path.join(import.meta.dirname ?? __dirname, "..", "css", "view.css");
    const source = await fs.readFile(cssPath, "utf-8");
    expect(source).toMatch(/\.status-bar\s*\{[\s\S]*?height:\s*26px/);
    expect(source).toMatch(/color:\s*var\(--text-faint\)/);
  });
});
