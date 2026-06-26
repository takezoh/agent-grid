// useTerminateSession.test.ts — 204 / 404 / 5xx / network の挙動 +
// 削除後の activeSessionID 切替.

import { act, renderHook } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import type { ApiHttpError, SessionsApi } from "../api/sessions";
import { useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";
import { useTerminateSession } from "./useTerminateSession";

function mockApi(deleteImpl: SessionsApi["deleteSession"]): SessionsApi {
  return {
    deleteSession: deleteImpl,
    createSession: vi.fn(),
    pushCommand: vi.fn(),
    getSessionConfig: vi.fn(),
  };
}

function httpError(status: number): ApiHttpError {
  const e = new Error(`HTTP ${status}`) as ApiHttpError;
  e.status = status;
  return e;
}

function seedSessions(active: string | null) {
  useDaemonStore.setState({
    sessions: [
      {
        id: "a",
        project: "p",
        command: "claude",
        created_at: "2026-06-20T00:00:00Z",
        view: { card: { title: "A" }, status: "running" },
      },
      {
        id: "b",
        project: "p",
        command: "claude",
        created_at: "2026-06-20T00:00:00Z",
        view: { card: { title: "B" }, status: "running" },
      },
    ],
    activeSessionID: active,
  });
}

describe("useTerminateSession — success path", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    useNotificationsStore.getState().clear();
  });

  it("204 success で true を返し、active なら次セッションへ切替", async () => {
    seedSessions("a");
    const api = mockApi(vi.fn().mockResolvedValue(undefined));
    const { result } = renderHook(() => useTerminateSession(api));

    let ok = false;
    await act(async () => {
      ok = await result.current.terminate("a");
    });

    expect(ok).toBe(true);
    expect(useDaemonStore.getState().activeSessionID).toBe("b");
  });

  it("active でない session を削除しても activeSessionID は変わらない", async () => {
    seedSessions("a");
    const api = mockApi(vi.fn().mockResolvedValue(undefined));
    const { result } = renderHook(() => useTerminateSession(api));

    await act(async () => {
      await result.current.terminate("b");
    });

    expect(useDaemonStore.getState().activeSessionID).toBe("a");
  });

  it("最後のセッションを削除すると active は null", async () => {
    useDaemonStore.setState({
      sessions: [
        {
          id: "a",
          project: "p",
          command: "claude",
          created_at: "2026-06-20T00:00:00Z",
          view: { card: { title: "A" }, status: "running" },
        },
      ],
      activeSessionID: "a",
    });
    const api = mockApi(vi.fn().mockResolvedValue(undefined));
    const { result } = renderHook(() => useTerminateSession(api));

    await act(async () => {
      await result.current.terminate("a");
    });

    expect(useDaemonStore.getState().activeSessionID).toBeNull();
  });
});

describe("useTerminateSession — error paths", () => {
  beforeEach(() => {
    useDaemonStore.getState().reset();
    useNotificationsStore.getState().clear();
    seedSessions("a");
  });

  it("404 は既に消えてるとみなし true を返す (silent close)", async () => {
    const api = mockApi(vi.fn().mockRejectedValue(httpError(404)));
    const { result } = renderHook(() => useTerminateSession(api));

    let ok = false;
    await act(async () => {
      ok = await result.current.terminate("a");
    });

    expect(ok).toBe(true);
    // active は次へ切り替わる
    expect(useDaemonStore.getState().activeSessionID).toBe("b");
    // toast は出さない
    expect(useNotificationsStore.getState().items.length).toBe(0);
  });

  it("500 は false を返し error toast を出す", async () => {
    const api = mockApi(vi.fn().mockRejectedValue(httpError(500)));
    const { result } = renderHook(() => useTerminateSession(api));

    let ok = true;
    await act(async () => {
      ok = await result.current.terminate("a");
    });

    expect(ok).toBe(false);
    // active は変わらない
    expect(useDaemonStore.getState().activeSessionID).toBe("a");
    const notes = useNotificationsStore.getState().items;
    expect(notes.length).toBe(1);
    expect(notes[0]?.level).toBe("error");
  });

  it("network error は false を返し error toast を出す", async () => {
    const api = mockApi(vi.fn().mockRejectedValue(new Error("network")));
    const { result } = renderHook(() => useTerminateSession(api));

    let ok = true;
    await act(async () => {
      ok = await result.current.terminate("a");
    });

    expect(ok).toBe(false);
    expect(useNotificationsStore.getState().items[0]?.message).toMatch(/ネットワーク/);
  });
});
