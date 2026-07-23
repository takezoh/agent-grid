import { beforeEach, describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
  selectFrameMessagingRevision,
  selectFrameMessagingSummary,
  summariesFromSessions,
  useFrameMessagingStore,
} from "./frameMessaging";

function makeSession(
  id: string,
  summary?: SessionInfo["view"]["frame_messaging_summary"],
): SessionInfo {
  return {
    id,
    project: `/repo/${id}`,
    command: "codex",
    created_at: "2026-07-06T00:00:00Z",
    view: {
      card: { title: id },
      ...(summary !== undefined ? { frame_messaging_summary: summary } : {}),
    },
  };
}

describe("frameMessaging store", () => {
  beforeEach(() => {
    useFrameMessagingStore.getState().reset();
  });

  it("summariesFromSessions materializes only sessions carrying a summary", () => {
    const got = summariesFromSessions([
      makeSession("s1", {
        unread_count: 2,
        latest_message_preview: "Need help",
        pending_delivery_count: 1,
        last_delivery_status: "pending",
      } as never),
      makeSession("s2"),
      makeSession("s3", {
        unread_count: 0,
        latest_reply_preview: "Done",
        pending_delivery_count: 0,
      } as never),
    ]);

    expect(got).toEqual({
      s1: {
        unreadCount: 2,
        latestMessagePreview: "Need help",
        pendingDeliveryCount: 1,
        lastDeliveryStatus: "pending",
      },
      s3: {
        unreadCount: 0,
        latestReplyPreview: "Done",
        pendingDeliveryCount: 0,
      },
    });
  });

  it("replaceFromSessions drops stale entries when a session disappears", () => {
    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s1", { unread_count: 1, pending_delivery_count: 0 } as never),
        makeSession("s2", { unread_count: 3, pending_delivery_count: 2 } as never),
      ]);

    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s2", { unread_count: 0, pending_delivery_count: 1 } as never),
      ]);

    expect(useFrameMessagingStore.getState().summaries).toEqual({
      s2: { unreadCount: 0, pendingDeliveryCount: 1 },
    });
  });

  it("replaceFromSessions removes an entry when the session no longer exposes a summary", () => {
    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s1", { unread_count: 4, pending_delivery_count: 1 } as never),
      ]);
    useFrameMessagingStore.getState().replaceFromSessions([makeSession("s1")]);

    expect(selectFrameMessagingSummary(useFrameMessagingStore.getState(), "s1")).toBeUndefined();
  });

  it("markRead zeros unreadCount without dropping the rest of the summary", () => {
    useFrameMessagingStore.getState().replaceFromSessions([
      makeSession("s1", {
        unread_count: 4,
        latest_message_preview: "Please check",
        pending_delivery_count: 1,
      } as never),
    ]);

    useFrameMessagingStore.getState().markRead("s1");

    expect(useFrameMessagingStore.getState().summaries).toEqual({
      s1: {
        unreadCount: 0,
        latestMessagePreview: "Please check",
        pendingDeliveryCount: 1,
      },
    });
    expect(selectFrameMessagingRevision(useFrameMessagingStore.getState(), "s1")).toBe(1);
  });

  it("replaceFromSessions increments revision only for server-side summary changes", () => {
    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s1", { unread_count: 1, pending_delivery_count: 0 } as never),
      ]);
    expect(selectFrameMessagingRevision(useFrameMessagingStore.getState(), "s1")).toBe(1);

    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s1", { unread_count: 1, pending_delivery_count: 0 } as never),
      ]);
    expect(selectFrameMessagingRevision(useFrameMessagingStore.getState(), "s1")).toBe(1);

    useFrameMessagingStore.getState().markRead("s1");
    expect(selectFrameMessagingRevision(useFrameMessagingStore.getState(), "s1")).toBe(1);

    useFrameMessagingStore
      .getState()
      .replaceFromSessions([
        makeSession("s1", { unread_count: 2, pending_delivery_count: 0 } as never),
      ]);
    expect(selectFrameMessagingRevision(useFrameMessagingStore.getState(), "s1")).toBe(2);
  });
});
