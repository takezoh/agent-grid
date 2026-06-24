import { fireEvent, render, screen } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { SessionList, displayLabel } from "./SessionList";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
  unsubscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

describe("displayLabel", () => {
  it("FR-011: returns title when title is present", () => {
    expect(displayLabel({ title: "My Session" }, "s1")).toBe("My Session");
  });

  it("FR-011: returns subtitle when title is absent", () => {
    expect(displayLabel({ subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-011: returns subtitle when title is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "sub" }, "s1")).toBe("sub");
  });

  it("FR-012: returns id when both title and subtitle are absent", () => {
    expect(displayLabel({}, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is undefined and subtitle is undefined", () => {
    expect(displayLabel({ title: undefined, subtitle: undefined }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is empty string and subtitle is empty string", () => {
    expect(displayLabel({ title: "", subtitle: "" }, "s1")).toBe("s1");
  });

  it("FR-012: returns id when title is whitespace-only and subtitle is whitespace-only", () => {
    expect(displayLabel({ title: "  ", subtitle: "   " }, "s1")).toBe("s1");
  });

  it("FR-011: trims title before returning it", () => {
    expect(displayLabel({ title: "  trimmed  " }, "s1")).toBe("trimmed");
  });

  it("FR-011: trims subtitle before returning it", () => {
    expect(displayLabel({ title: "", subtitle: "  sub  " }, "s1")).toBe("sub");
  });
});

describe("SessionList rendering", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
  });

  it("renders session with title via displayLabel", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("alpha")).toBeDefined();
  });

  it("renders session with subtitle when title is absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { subtitle: "my-sub" }, status: "running" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("my-sub")).toBeDefined();
  });

  it("FR-012: renders session id when both title and subtitle are absent", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-raw-id",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: {}, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-raw-id")).toBeDefined();
  });

  it("renders session id when title and subtitle are empty strings", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-empty",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "", subtitle: "" }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-empty")).toBeDefined();
  });

  it("renders session id when title and subtitle are whitespace-only", () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "s-ws",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "  ", subtitle: "  " }, status: "stopped" },
        },
      ],
    });
    render(<SessionList conn={fakeConn} />);
    expect(screen.getByText("s-ws")).toBeDefined();
  });
});

describe("SessionList onClick", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    vi.clearAllMocks();
    useDaemonStore.setState({
      sessions: [
        {
          id: "s1",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "alpha" }, status: "running" },
        },
        {
          id: "s2",
          project: "proj",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "beta" }, status: "stopped" },
        },
      ],
    });
  });

  it("calls selectSession on click", () => {
    const selectSession = vi.fn();
    useDaemonStore.setState({ selectSession });
    render(<SessionList conn={fakeConn} />);
    fireEvent.click(screen.getByText("beta"));
    expect(selectSession).toHaveBeenCalledWith("s2");
  });

  it("ADR-0030: does NOT call conn.subscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    fireEvent.click(screen.getByText("beta"));
    expect((fakeConn.subscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });

  it("ADR-0030: does NOT call conn.unsubscribe on click", () => {
    useDaemonStore.setState({ activeSessionID: "s1" });
    render(<SessionList conn={fakeConn} />);
    fireEvent.click(screen.getByText("beta"));
    expect((fakeConn.unsubscribe as ReturnType<typeof vi.fn>).mock.calls).toHaveLength(0);
  });
});
