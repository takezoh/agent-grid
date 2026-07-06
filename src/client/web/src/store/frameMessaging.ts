import { create } from "zustand";
import type { FrameMessagingSummary, SessionInfo } from "../wire/server";

export type FrameMessagingSummaryMap = Record<string, FrameMessagingSummary>;

export type FrameMessagingState = {
  summaries: FrameMessagingSummaryMap;
  replaceFromSessions: (sessions: readonly SessionInfo[]) => void;
  reset: () => void;
};

export function summariesFromSessions(sessions: readonly SessionInfo[]): FrameMessagingSummaryMap {
  const next: FrameMessagingSummaryMap = {};
  for (const session of sessions) {
    const summary = session.view.frame_messaging_summary;
    if (!summary) continue;
    next[session.id] = {
      unreadCount: summary.unreadCount,
      pendingDeliveryCount: summary.pendingDeliveryCount,
      ...(summary.latestMessagePreview !== undefined
        ? { latestMessagePreview: summary.latestMessagePreview }
        : {}),
      ...(summary.latestReplyPreview !== undefined
        ? { latestReplyPreview: summary.latestReplyPreview }
        : {}),
      ...(summary.lastDeliveryStatus !== undefined
        ? { lastDeliveryStatus: summary.lastDeliveryStatus }
        : {}),
    };
  }
  return next;
}

export function selectFrameMessagingSummary(
  state: FrameMessagingState,
  sessionId: string,
): FrameMessagingSummary | undefined {
  return state.summaries[sessionId];
}

export const useFrameMessagingStore = create<FrameMessagingState>()((set) => ({
  summaries: {},
  replaceFromSessions: (sessions) => set({ summaries: summariesFromSessions(sessions) }),
  reset: () => set({ summaries: {} }),
}));
