import { act, render, screen, waitFor } from "@testing-library/react";
import { beforeEach, describe, expect, it, vi } from "vitest";
import { selectFrameMessagingSummary, useFrameMessagingStore } from "../store/frameMessaging";
import { MessagesPanel } from "./MessagesPanel";

function ConnectedMessagesPanel({
  sessionId,
  fetchFn,
}: {
  sessionId: string;
  fetchFn: typeof fetch;
}) {
  const summary = useFrameMessagingStore((state) => selectFrameMessagingSummary(state, sessionId));
  if (!summary) throw new Error(`missing summary for ${sessionId}`);
  return <MessagesPanel sessionId={sessionId} summary={summary} fetchFn={fetchFn} />;
}

describe("MessagesPanel", () => {
  beforeEach(() => {
    useFrameMessagingStore.getState().reset();
    vi.clearAllMocks();
    window.location.hash = "#token=test";
  });

  it("loads messages and marks the session read when unread_count > 0", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            session_id: "s1",
            summary: { unread_count: 2, pending_delivery_count: 1 },
            messages: [
              {
                id: "m1",
                source_frame_id: "frame-a",
                target_frame_id: "frame-b",
                topic: "Need review",
                body_preview: "Please review the patch",
                reply_status: "pending",
                created_at: "2026-07-06T00:00:00Z",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response("", { status: 204 })) as unknown as typeof fetch;

    useFrameMessagingStore.setState({
      summaries: {
        s1: { unreadCount: 2, pendingDeliveryCount: 1 },
      },
    });

    render(
      <MessagesPanel
        sessionId="s1"
        summary={{ unreadCount: 2, pendingDeliveryCount: 1 }}
        fetchFn={fetchFn}
      />,
    );

    expect(await screen.findByText("Need review")).toBeTruthy();
    expect(screen.getByText("Please review the patch")).toBeTruthy();

    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalledTimes(2);
    });
    const markReadCall = vi.mocked(fetchFn).mock.calls[1];
    expect(markReadCall?.[1]?.body).toBe(`{"last_read_message_id":"m1"}`);
    expect(useFrameMessagingStore.getState().summaries.s1?.unreadCount).toBe(0);
  });

  it("keeps the loaded message list visible when mark-read fails", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            session_id: "s1",
            summary: { unread_count: 2, pending_delivery_count: 1 },
            messages: [
              {
                id: "m1",
                source_frame_id: "frame-a",
                target_frame_id: "frame-b",
                topic: "Need review",
                body_preview: "Please review the patch",
                reply_status: "pending",
                created_at: "2026-07-06T00:00:00Z",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response("boom", { status: 500 })) as unknown as typeof fetch;

    useFrameMessagingStore.setState({
      summaries: {
        s1: { unreadCount: 2, pendingDeliveryCount: 1 },
      },
    });

    render(
      <MessagesPanel
        sessionId="s1"
        summary={{ unreadCount: 2, pendingDeliveryCount: 1 }}
        fetchFn={fetchFn}
      />,
    );

    expect(await screen.findByText("Need review")).toBeTruthy();
    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalledTimes(2);
    });
    const markReadCall = vi.mocked(fetchFn).mock.calls[1];
    expect(markReadCall?.[1]?.body).toBe(`{"last_read_message_id":"m1"}`);
    expect(screen.getByText("Please review the patch")).toBeTruthy();
    expect(screen.queryByText(/Failed to load messages/)).toBeNull();
    expect(useFrameMessagingStore.getState().summaries.s1?.unreadCount).toBe(2);
  });

  it("does not refetch immediately after local unread count becomes zero", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            session_id: "s1",
            summary: { unread_count: 2, pending_delivery_count: 1 },
            messages: [
              {
                id: "m1",
                source_frame_id: "frame-a",
                target_frame_id: "frame-b",
                topic: "Need review",
                body_preview: "Please review the patch",
                reply_status: "pending",
                created_at: "2026-07-06T00:00:00Z",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response("", { status: 204 })) as unknown as typeof fetch;

    useFrameMessagingStore.setState({
      summaries: {
        s1: { unreadCount: 2, pendingDeliveryCount: 1 },
      },
    });

    render(<ConnectedMessagesPanel sessionId="s1" fetchFn={fetchFn} />);

    expect(await screen.findByText("Need review")).toBeTruthy();
    await waitFor(() => {
      expect(useFrameMessagingStore.getState().summaries.s1?.unreadCount).toBe(0);
    });
    expect(fetchFn).toHaveBeenCalledTimes(2);
    expect(screen.queryByText("Loading messages…")).toBeNull();
  });

  it("refetches when the same session receives a new server summary", async () => {
    const fetchFn = vi
      .fn()
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            session_id: "s1",
            summary: { unread_count: 0, pending_delivery_count: 0 },
            messages: [
              {
                id: "m1",
                source_frame_id: "frame-a",
                target_frame_id: "frame-b",
                topic: "Old message",
                body_preview: "old body",
                reply_status: "pending",
                created_at: "2026-07-06T00:00:00Z",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(
        new Response(
          JSON.stringify({
            session_id: "s1",
            summary: { unread_count: 1, pending_delivery_count: 0 },
            messages: [
              {
                id: "m1",
                source_frame_id: "frame-a",
                target_frame_id: "frame-b",
                topic: "Old message",
                body_preview: "old body",
                reply_status: "pending",
                created_at: "2026-07-06T00:00:00Z",
              },
              {
                id: "m2",
                source_frame_id: "frame-c",
                target_frame_id: "frame-d",
                topic: "New message",
                body_preview: "new body",
                reply_status: "pending",
                created_at: "2026-07-06T00:01:00Z",
              },
            ],
          }),
          { status: 200, headers: { "Content-Type": "application/json" } },
        ),
      )
      .mockResolvedValueOnce(new Response("", { status: 204 })) as unknown as typeof fetch;

    useFrameMessagingStore.setState({
      summaries: {
        s1: { unreadCount: 0, pendingDeliveryCount: 0 },
      },
      revisions: {
        s1: 1,
      },
    });

    render(<ConnectedMessagesPanel sessionId="s1" fetchFn={fetchFn} />);

    expect(await screen.findByText("Old message")).toBeTruthy();
    expect(fetchFn).toHaveBeenCalledTimes(1);

    act(() => {
      useFrameMessagingStore.getState().replaceFromSessions([
        {
          id: "s1",
          project: "/repo/s1",
          command: "codex",
          created_at: "2026-07-06T00:00:00Z",
          view: {
            card: { title: "s1" },
            frame_messaging_summary: {
              unread_count: 1,
              pending_delivery_count: 0,
            },
          },
        },
      ]);
    });

    expect(await screen.findByText("New message")).toBeTruthy();
    await waitFor(() => {
      expect(fetchFn).toHaveBeenCalledTimes(3);
    });
    expect(screen.getByText("new body")).toBeTruthy();
  });
});
