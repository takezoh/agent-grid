import { beforeEach, describe, expect, it } from "vitest";
import type { SessionInfo } from "../wire/server";
import {
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
        unreadCount: 2,
        latestMessagePreview: "Need help",
        pendingDeliveryCount: 1,
        lastDeliveryStatus: "pending",
      }),
      makeSession("s2"),
      makeSession("s3", {
        unreadCount: 0,
        latestReplyPreview: "Done",
        pendingDeliveryCount: 0,
      }),
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
        makeSession("s1", { unreadCount: 1, pendingDeliveryCount: 0 }),
        makeSession("s2", { unreadCount: 3, pendingDeliveryCount: 2 }),
      ]);

    useFrameMessagingStore
      .getState()
      .replaceFromSessions([makeSession("s2", { unreadCount: 0, pendingDeliveryCount: 1 })]);

    expect(useFrameMessagingStore.getState().summaries).toEqual({
      s2: { unreadCount: 0, pendingDeliveryCount: 1 },
    });
  });

  it("replaceFromSessions removes an entry when the session no longer exposes a summary", () => {
    useFrameMessagingStore
      .getState()
      .replaceFromSessions([makeSession("s1", { unreadCount: 4, pendingDeliveryCount: 1 })]);
    useFrameMessagingStore.getState().replaceFromSessions([makeSession("s1")]);

    expect(selectFrameMessagingSummary(useFrameMessagingStore.getState(), "s1")).toBeUndefined();
  });
});
