import type { ClientFrame } from "../wire/client";
import type { RespErrFrame, RespOKFrame, ServerFrame } from "../wire/server";
import { type Rng, backoffDelay, exceededAttempts } from "./backoff";

export type SubscribeOutcome =
  | { status: "confirmed"; reqId: string }
  | { status: "exhausted"; lastError: string };

// Caller-provided side effects.
export type RetryDeps = {
  /**
   * Hands a frame to the socket. Takes the frame, not a serialized string:
   * serialization is the wire owner's job, and a string-taking `send` was an
   * escape hatch that let a caller assemble and serialize a frame outside
   * Connection.
   */
  send: (frame: ClientFrame) => void;
  // Awaits the next Resp frame matching reqId. Implementations: see connection.ts
  // which routes incoming frames to a per-reqId promise registry.
  awaitResponse: (reqId: string) => Promise<RespOKFrame | RespErrFrame>;
  newReqId: () => string;
  // sleep wrapper, injectable for tests
  sleep: (ms: number) => Promise<void>;
  rng?: Rng;
};

export async function subscribeWithRetry(
  sessionId: string,
  cols: number,
  rows: number,
  deps: RetryDeps,
): Promise<SubscribeOutcome> {
  let attempt = 0;
  let lastError = "";
  while (!exceededAttempts(attempt)) {
    const reqId = deps.newReqId();
    deps.send({ k: "s", reqId, sessionId, cols, rows });
    const resp = await deps.awaitResponse(reqId);
    if (resp.k === "r") {
      return { status: "confirmed", reqId };
    }
    // resp.k === "e"
    if (resp.code !== "frame-not-ready") {
      // non-retryable
      return { status: "exhausted", lastError: resp.code };
    }
    lastError = resp.code;
    attempt += 1;
    if (exceededAttempts(attempt)) break;
    await deps.sleep(backoffDelay(attempt, deps.rng));
  }
  return { status: "exhausted", lastError };
}

// helper for tests: classify a ServerFrame as Resp matching a reqId
export function matchResp(frame: ServerFrame, reqId: string): RespOKFrame | RespErrFrame | null {
  if (Array.isArray(frame)) return null;
  if (frame.k === "r" && frame.reqId === reqId) return frame;
  if (frame.k === "e" && frame.reqId === reqId) return frame;
  return null;
}
