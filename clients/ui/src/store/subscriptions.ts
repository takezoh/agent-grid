import { create } from "zustand";

export type TerminalSubscriptionPhase =
  | "idle"
  | "disconnected"
  | "subscribing"
  | "waiting"
  | "confirmed"
  | "blocked";

export type TerminalSubscriptionSnapshot = {
  sessionId: string | null;
  phase: TerminalSubscriptionPhase;
  attempt: number;
  lastError: string | null;
  ownershipEpoch: number;
};

type SubscriptionState = TerminalSubscriptionSnapshot & {
  replace: (snapshot: TerminalSubscriptionSnapshot) => void;
  reset: () => void;
};

export const initialTerminalSubscriptionSnapshot: TerminalSubscriptionSnapshot = {
  sessionId: null,
  phase: "idle",
  attempt: 0,
  lastError: null,
  ownershipEpoch: 0,
};

export const useSubscriptionStore = create<SubscriptionState>()((set) => ({
  ...initialTerminalSubscriptionSnapshot,
  replace: (snapshot) => set(snapshot),
  reset: () => set(initialTerminalSubscriptionSnapshot),
}));
