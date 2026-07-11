import { create } from "zustand";
import type { FrameMessagingSummary, SessionInfo } from "../wire/server";

export type NormalizedFrameMessagingSummary = {
  unreadCount: number;
  latestMessagePreview?: string;
  latestReplyPreview?: string;
  pendingDeliveryCount: number;
  lastDeliveryStatus?: string;
};

export type FrameMessagingSummaryMap = Record<string, NormalizedFrameMessagingSummary>;
export type FrameMessagingRevisionMap = Record<string, number>;

export type FrameMessagingState = {
  summaries: FrameMessagingSummaryMap;
  revisions: FrameMessagingRevisionMap;
  replaceFromSessions: (sessions: readonly SessionInfo[]) => void;
  markRead: (sessionId: string) => void;
  reset: () => void;
};

export function normalizeFrameMessagingSummary(
  summary: FrameMessagingSummary | undefined,
): NormalizedFrameMessagingSummary | undefined {
  if (!summary) return undefined;
  const unreadCount = summary.unread_count ?? summary.unreadCount;
  const pendingDeliveryCount = summary.pending_delivery_count ?? summary.pendingDeliveryCount;
  if (typeof unreadCount !== "number" || typeof pendingDeliveryCount !== "number") {
    return undefined;
  }
  return {
    unreadCount,
    pendingDeliveryCount,
    ...(summary.latest_message_preview !== undefined
      ? { latestMessagePreview: summary.latest_message_preview }
      : summary.latestMessagePreview !== undefined
        ? { latestMessagePreview: summary.latestMessagePreview }
        : {}),
    ...(summary.latest_reply_preview !== undefined
      ? { latestReplyPreview: summary.latest_reply_preview }
      : summary.latestReplyPreview !== undefined
        ? { latestReplyPreview: summary.latestReplyPreview }
        : {}),
    ...(summary.last_delivery_status !== undefined
      ? { lastDeliveryStatus: summary.last_delivery_status }
      : summary.lastDeliveryStatus !== undefined
        ? { lastDeliveryStatus: summary.lastDeliveryStatus }
        : {}),
  };
}

export function summariesFromSessions(sessions: readonly SessionInfo[]): FrameMessagingSummaryMap {
  const next: FrameMessagingSummaryMap = {};
  for (const session of sessions) {
    const normalized = normalizeFrameMessagingSummary(session.view.frame_messaging_summary);
    if (!normalized) continue;
    next[session.id] = normalized;
  }
  return next;
}

export function selectFrameMessagingSummary(
  state: FrameMessagingState,
  sessionId: string,
): NormalizedFrameMessagingSummary | undefined {
  return state.summaries[sessionId];
}

export function selectFrameMessagingRevision(
  state: FrameMessagingState,
  sessionId: string,
): number {
  return state.revisions[sessionId] ?? 0;
}

export const useFrameMessagingStore = create<FrameMessagingState>()((set) => ({
  summaries: {},
  revisions: {},
  replaceFromSessions: (sessions) =>
    set((state) => {
      const nextSummaries = summariesFromSessions(sessions);
      const nextRevisions: FrameMessagingRevisionMap = {};
      for (const [sessionID, nextSummary] of Object.entries(nextSummaries)) {
        const prevSummary = state.summaries[sessionID];
        const prevRevision = state.revisions[sessionID] ?? 0;
        nextRevisions[sessionID] = summariesEqual(prevSummary, nextSummary)
          ? prevRevision
          : prevRevision + 1;
      }
      return { summaries: nextSummaries, revisions: nextRevisions };
    }),
  markRead: (sessionId) =>
    set((s) => {
      const current = s.summaries[sessionId];
      if (!current || current.unreadCount === 0) return s;
      return {
        summaries: {
          ...s.summaries,
          [sessionId]: {
            ...current,
            unreadCount: 0,
          },
        },
        revisions: s.revisions,
      };
    }),
  reset: () => set({ summaries: {}, revisions: {} }),
}));

function summariesEqual(
  left: NormalizedFrameMessagingSummary | undefined,
  right: NormalizedFrameMessagingSummary | undefined,
): boolean {
  if (left === right) return true;
  if (!left || !right) return false;
  return (
    left.unreadCount === right.unreadCount &&
    left.latestMessagePreview === right.latestMessagePreview &&
    left.latestReplyPreview === right.latestReplyPreview &&
    left.pendingDeliveryCount === right.pendingDeliveryCount &&
    left.lastDeliveryStatus === right.lastDeliveryStatus
  );
}
