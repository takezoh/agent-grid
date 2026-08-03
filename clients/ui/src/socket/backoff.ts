// Exponential backoff with full jitter. Shared by reconnect (connection.ts) and
// subscribe retry (retry.ts) per ADR 0022.

export const INITIAL_MS = 250;
export const CAP_MS = 2000;
export const MAX_ATTEMPTS = 16;

export type Rng = () => number; // 0..1, defaults to Math.random

export type DelayOutcome = "fired" | "canceled";

export type CancelableDelay = {
  promise: Promise<DelayOutcome>;
  cancel: () => void;
};

const defaultSleep = (ms: number): Promise<void> =>
  new Promise((resolve) => {
    setTimeout(resolve, ms);
  });

export function backoffDelay(attempt: number, rng: Rng = Math.random): number {
  if (attempt < 0) return 0;
  const exp = Math.min(CAP_MS, INITIAL_MS * 2 ** attempt);
  // full jitter: random in [0, exp]
  return Math.floor(rng() * exp);
}

export function exceededAttempts(attempt: number): boolean {
  return attempt >= MAX_ATTEMPTS;
}

/** Schedule a delay that Connection can cancel without relying on wall-clock waits. */
export function scheduleDelay(
  ms: number,
  sleep: (ms: number) => Promise<void> = defaultSleep,
): CancelableDelay {
  let settled = false;
  let resolve!: (outcome: DelayOutcome) => void;
  const promise = new Promise<DelayOutcome>((r) => {
    resolve = r;
  });
  const settle = (outcome: DelayOutcome): void => {
    if (settled) return;
    settled = true;
    resolve(outcome);
  };
  void sleep(ms).then(
    () => settle("fired"),
    () => settle("fired"),
  );
  return {
    promise,
    cancel: () => settle("canceled"),
  };
}
