// SessionContextMenu.test.tsx
//
// Right-click on a session row opens a session-scoped action menu:
//   - Open           → onOpen(sessionId)
//   - Copy session ID → clipboard + toast
//   - Stop session…  → onRequestTerminate(id, label, opener)
// Integration through SessionList (real UnifiedListbox rows) is covered at
// the bottom; the SessionContextMenu-only cases mount a plain trigger div.

import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";
import { SessionContextMenu } from "./SessionContextMenu";
import { SessionList } from "./SessionList";

const fakeConn = {
  subscribe: vi.fn(async () => {}),
  unsubscribe: vi.fn(async () => {}),
} as unknown as import("../socket/connection").Connection;

interface MountArgs {
  daemonDisconnected?: boolean;
  onOpen?: (id: string) => void;
  onRequestTerminate?: (id: string, label: string, opener: HTMLElement) => void;
}

function mountMenu({
  daemonDisconnected = false,
  onOpen = vi.fn(),
  onRequestTerminate,
}: MountArgs = {}) {
  const utils = render(
    <SessionContextMenu
      sessionId="s1"
      sessionLabel="alpha"
      daemonDisconnected={daemonDisconnected}
      onOpen={onOpen}
      onRequestTerminate={onRequestTerminate}
    >
      <div data-testid="row">alpha row</div>
    </SessionContextMenu>,
  );
  return { row: screen.getByTestId("row"), ...utils };
}

beforeEach(() => {
  useNotificationsStore.setState({ items: [], nextId: 1, muted: false });
});

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
});

describe("SessionContextMenu — open on right-click", () => {
  it("contextmenu on the trigger shows Open / Copy session ID / Stop session items", async () => {
    mountMenu({ onRequestTerminate: vi.fn() });
    fireEvent.contextMenu(screen.getByTestId("row"));

    expect(await screen.findByText("Open")).toBeTruthy();
    expect(screen.getByText("Copy session ID")).toBeTruthy();
    expect(screen.getByText("Stop session…")).toBeTruthy();
    // Menu is labelled with the session title so the target is unambiguous.
    expect(screen.getByText("alpha")).toBeTruthy();
  });

  it("omits Stop session when no terminate handler is wired", async () => {
    mountMenu();
    fireEvent.contextMenu(screen.getByTestId("row"));

    expect(await screen.findByText("Open")).toBeTruthy();
    expect(screen.queryByText("Stop session…")).toBeNull();
  });
});

describe("SessionContextMenu — actions", () => {
  it("Open calls onOpen with the session id", async () => {
    const onOpen = vi.fn();
    mountMenu({ onOpen });
    fireEvent.contextMenu(screen.getByTestId("row"));

    const item = await screen.findByText("Open");
    act(() => {
      fireEvent.click(item);
    });

    expect(onOpen).toHaveBeenCalledTimes(1);
    expect(onOpen).toHaveBeenCalledWith("s1");
  });

  it("Stop session… calls onRequestTerminate with id, label and an opener element", async () => {
    const onRequestTerminate = vi.fn();
    mountMenu({ onRequestTerminate });
    fireEvent.contextMenu(screen.getByTestId("row"));

    const item = await screen.findByText("Stop session…");
    act(() => {
      fireEvent.click(item);
    });

    expect(onRequestTerminate).toHaveBeenCalledTimes(1);
    const [id, label, opener] = onRequestTerminate.mock.calls[0] as [string, string, HTMLElement];
    expect(id).toBe("s1");
    expect(label).toBe("alpha");
    // opener is the trigger row itself — the focus-restore target for the
    // ConfirmDialog (WCAG 2.4.3).
    expect(opener).toBe(screen.getByTestId("row"));
  });

  it("Copy session ID writes to the clipboard and raises an info toast", async () => {
    const writeText = vi.fn(async () => {});
    vi.stubGlobal("navigator", { ...navigator, clipboard: { writeText } });

    mountMenu();
    fireEvent.contextMenu(screen.getByTestId("row"));

    const item = await screen.findByText("Copy session ID");
    await act(async () => {
      fireEvent.click(item);
    });

    expect(writeText).toHaveBeenCalledWith("s1");
    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(1);
    expect(items[0]?.level).toBe("info");
    expect(items[0]?.message).toContain("s1");
    vi.unstubAllGlobals();
  });

  it("Copy session ID falls back to a warn toast when clipboard is unavailable", async () => {
    vi.stubGlobal("navigator", { ...navigator, clipboard: undefined });

    mountMenu();
    fireEvent.contextMenu(screen.getByTestId("row"));

    const item = await screen.findByText("Copy session ID");
    await act(async () => {
      fireEvent.click(item);
    });

    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(1);
    expect(items[0]?.level).toBe("warn");
    expect(items[0]?.message).toContain("s1");
    vi.unstubAllGlobals();
  });

  it("daemonDisconnected disables Open and Stop session", async () => {
    const onOpen = vi.fn();
    const onRequestTerminate = vi.fn();
    mountMenu({ daemonDisconnected: true, onOpen, onRequestTerminate });
    fireEvent.contextMenu(screen.getByTestId("row"));

    const open = await screen.findByText("Open");
    const stop = screen.getByText("Stop session…");
    expect(open.getAttribute("data-disabled")).not.toBeNull();
    expect(stop.getAttribute("data-disabled")).not.toBeNull();

    act(() => {
      fireEvent.click(open);
      fireEvent.click(stop);
    });
    expect(onOpen).not.toHaveBeenCalled();
    expect(onRequestTerminate).not.toHaveBeenCalled();
  });
});

// ---------------------------------------------------------------------------
// Integration through SessionList: right-click on a real row
// ---------------------------------------------------------------------------

describe("SessionList — right-click on a session row", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
  });

  function seedSession() {
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
  }

  it("opens the context menu for the clicked session", async () => {
    seedSession();
    const onRequestTerminate = vi.fn();
    const { container } = render(
      <SessionList conn={fakeConn} onRequestTerminate={onRequestTerminate} />,
    );

    const row = container.querySelector('[data-session-id="s1"]');
    expect(row).not.toBeNull();
    fireEvent.contextMenu(row as HTMLElement);

    const stop = await screen.findByText("Stop session…");
    act(() => {
      fireEvent.click(stop);
    });
    expect(onRequestTerminate).toHaveBeenCalledWith("s1", "alpha", expect.any(HTMLElement));
  });

  it("right-click does not select (activate) the session", () => {
    seedSession();
    const { container } = render(<SessionList conn={fakeConn} />);

    const option = container.querySelector('[data-item-id="s1"]');
    expect(option).not.toBeNull();
    fireEvent.pointerDown(option as HTMLElement, { button: 2 });
    fireEvent.contextMenu(option as HTMLElement);

    expect(useDaemonStore.getState().activeSessionID).toBeNull();
  });
});
