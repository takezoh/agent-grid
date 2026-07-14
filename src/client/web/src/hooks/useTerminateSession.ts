// useTerminateSession — stop button -> confirm dialog -> deleteSession hook.
//
// Responsibilities:
//   1. Call /api/sessions/{id} DELETE
//   2. Treat 204 / 404 (= already gone) as success and close
//   3. 5xx / network shows an error toast and keeps the dialog open (pending=false)
//   4. When the deleted session was active, switch activeSessionID to the
//      next session via selectNextActiveAfterDelete (ADR-0030: view-updates
//      no longer carry activeSessionID, so the web side must be explicit)
//
// SessionsApi is swappable via an optional argument for testability.

import { useCallback, useMemo, useState } from "react";
import { type ApiHttpError, type SessionsApi, makeSessionsApi } from "../api/sessions";
import { selectNextActiveAfterDelete, useDaemonStore } from "../store/daemon";
import { useNotificationsStore } from "../store/notifications";

export interface UseTerminateSessionResult {
  /** Call on confirm. true=dialog may close / false=stays open (error). */
  terminate: (id: string) => Promise<boolean>;
  /** API in-flight. Drives confirm button disabled / label swap. */
  pending: boolean;
}

function isHttpError(e: unknown): e is ApiHttpError {
  return e instanceof Error && typeof (e as ApiHttpError).status === "number";
}

function isNetworkError(e: unknown): boolean {
  return e instanceof Error && e.message === "network";
}

export function useTerminateSession(api?: SessionsApi): UseTerminateSessionResult {
  const [pending, setPending] = useState(false);
  // makeSessionsApi() carries an internal bearerMissingNotified one-shot flag
  // (see the comment at api/sessions.ts:128). Pinning one instance for the
  // hook lifetime prevents the missing-token warn firing on every click.
  const sessionsApi = useMemo(() => api ?? makeSessionsApi(), [api]);

  const terminate = useCallback(
    async (id: string): Promise<boolean> => {
      // Pin the pre-delete snapshot before awaiting. View-updates arrive
      // independently over WS, so a post-await getState() may already have
      // deletedId removed from sessions — in that race
      // selectNextActiveAfterDelete returns null at its first guard and the
      // surviving sibling sessions are never selected (blank screen).
      const preSessions = useDaemonStore.getState().sessions;
      const preActiveId = useDaemonStore.getState().activeSessionID;
      setPending(true);
      let succeeded = false;
      try {
        await sessionsApi.deleteSession(id);
        succeeded = true;
      } catch (e) {
        if (isHttpError(e) && e.status === 404) {
          // Already gone — the desired state, treat as success
          succeeded = true;
        } else if (isHttpError(e)) {
          useNotificationsStore.getState().add({
            level: "error",
            message: `Failed to stop session (HTTP ${e.status})`,
          });
        } else if (isNetworkError(e)) {
          useNotificationsStore.getState().add({
            level: "error",
            message: "Failed to stop session (network error)",
          });
        } else {
          useNotificationsStore.getState().add({
            level: "error",
            message: "Failed to stop session",
          });
        }
      }

      if (succeeded && preActiveId === id) {
        const next = selectNextActiveAfterDelete(preSessions, id);
        useDaemonStore.getState().selectSession(next);
      }

      setPending(false);
      return succeeded;
    },
    [sessionsApi],
  );

  return { terminate, pending };
}
