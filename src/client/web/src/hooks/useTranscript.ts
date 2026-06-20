import { useEffect } from "react";
import { type TranscriptKindParam, fetchSessionFile, splitLines } from "../api/transcripts";
import { useTranscriptStore } from "../store/transcripts";

export type UseTranscriptOpts = {
  sessionId: string;
  kind: TranscriptKindParam;
  bearerToken: string;
  // テスト注入用
  fetchFn?: typeof fetch;
};

export function useTranscript(opts: UseTranscriptOpts): void {
  useEffect(() => {
    let cancelled = false;
    (async () => {
      const result = await fetchSessionFile(opts.sessionId, opts.kind, 0, {
        bearerToken: opts.bearerToken,
        fetchFn: opts.fetchFn,
      });
      if (cancelled) return;
      if (result.status === "ok") {
        const lines = splitLines(result.text);
        useTranscriptStore
          .getState()
          .appendBackfill(opts.sessionId, opts.kind, lines, result.nextOffset);
      }
      // empty / not-modified: no-op
    })().catch(() => {
      /* swallow — toast 通知は別 PR */
    });
    return () => {
      cancelled = true;
    };
  }, [opts.sessionId, opts.kind, opts.bearerToken, opts.fetchFn]);
}
