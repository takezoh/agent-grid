import { useEffect, useState } from "react";
import { makeSessionsApi, type SessionMessage } from "../api/sessions";
import type { FrameMessagingSummary } from "../wire/server";
import { selectFrameMessagingRevision, useFrameMessagingStore } from "../store/frameMessaging";
import "../css/messages-panel.css";

export type MessagesPanelProps = {
  sessionId: string;
  summary: FrameMessagingSummary;
  fetchFn?: typeof fetch;
};

type LoadState =
  | { kind: "loading" }
  | { kind: "ready"; messages: SessionMessage[] }
  | { kind: "error"; message: string };

export function MessagesPanel({ sessionId, summary, fetchFn }: MessagesPanelProps) {
  const [state, setState] = useState<LoadState>({ kind: "loading" });
  const serverRevision = useFrameMessagingStore((store) =>
    selectFrameMessagingRevision(store, sessionId),
  );

  useEffect(() => {
    let cancelled = false;
    const api = makeSessionsApi(fetchFn);
    setState({ kind: "loading" });
    void api
      .getSessionMessages(sessionId)
      .then((resp) => {
        if (cancelled) return;
        setState({ kind: "ready", messages: resp.messages });
        const lastVisibleMessage = resp.messages[resp.messages.length - 1];
        const lastVisibleMessageId = lastVisibleMessage ? lastVisibleMessage.id : undefined;
        const unreadCount =
          resp.summary?.unread_count ?? summary.unread_count ?? summary.unreadCount ?? 0;
        if (unreadCount > 0 && lastVisibleMessageId) {
          void api
            .markSessionMessagesRead(sessionId, lastVisibleMessageId)
            .then(() => {
              if (!cancelled) {
                useFrameMessagingStore.getState().markRead(sessionId);
              }
            })
            .catch(() => {
              // Read-marking is best effort. Keep the loaded message list visible.
            });
        }
      })
      .catch((err: unknown) => {
        if (cancelled) return;
        const message = err instanceof Error ? err.message : String(err);
        setState({ kind: "error", message });
      });
    return () => {
      cancelled = true;
    };
  }, [fetchFn, sessionId, serverRevision]);

  return (
    <section className="messages-panel" aria-label="messages panel">
      <header className="messages-panel__summary">
        <span className="messages-pill">Unread {summary.unreadCount}</span>
        <span className="messages-pill">Pending {summary.pendingDeliveryCount}</span>
        {summary.lastDeliveryStatus && (
          <span className="messages-pill">Last {summary.lastDeliveryStatus}</span>
        )}
      </header>
      {state.kind === "loading" && <div className="messages-empty">Loading messages…</div>}
      {state.kind === "error" && (
        <div className="messages-empty">Failed to load messages: {state.message}</div>
      )}
      {state.kind === "ready" && state.messages.length === 0 && (
        <div className="messages-empty">No messages</div>
      )}
      {state.kind === "ready" && state.messages.length > 0 && (
        <div className="messages-list">
          {state.messages.map((message) => (
            <article key={message.id} className="messages-card">
              <div className="messages-card__meta">
                <span>{message.source_frame_id}</span>
                <span>{message.target_frame_id}</span>
                <span>{message.reply_status ?? "pending"}</span>
              </div>
              {message.topic && <h3 className="messages-card__topic">{message.topic}</h3>}
              <p className="messages-card__preview">{message.body_preview ?? message.body ?? ""}</p>
              {message.final_answer_preview && (
                <p className="messages-card__final">Final: {message.final_answer_preview}</p>
              )}
              {message.reply?.body_preview && (
                <p className="messages-card__reply">Reply: {message.reply.body_preview}</p>
              )}
            </article>
          ))}
        </div>
      )}
    </section>
  );
}
