import { create } from "zustand";
import type { HelloFrame, SessionInfo, ViewUpdateFrame } from "../wire/server";

export type ConnectionStatus = "connecting" | "open" | "reconnecting" | "closed";

export type DaemonState = {
  sessions: SessionInfo[];
  activeSessionID: string | null;
  connectors: string[]; // β は空配列で seed のみ。δ で活用。
  features: string[];
  serverTime: number;
  status: ConnectionStatus;
  // control frame で daemon-disconnected が来たかどうか。StatusBanner が参照。
  daemonDisconnected: boolean;

  // actions
  seedHello: (frame: HelloFrame) => void;
  applyViewUpdate: (frame: ViewUpdateFrame) => void;
  selectSession: (id: string | null) => void;
  setStatus: (status: ConnectionStatus) => void;
  setDaemonDisconnected: (v: boolean) => void;
  reset: () => void;
};

const initialState = {
  sessions: [] as SessionInfo[],
  activeSessionID: null as string | null,
  connectors: [] as string[],
  features: [] as string[],
  serverTime: 0,
  status: "connecting" as ConnectionStatus,
  daemonDisconnected: false,
};

export const useDaemonStore = create<DaemonState>()((set) => ({
  ...initialState,
  seedHello: (frame) =>
    set({
      sessions: frame.sessions,
      activeSessionID: frame.activeSessionID,
      features: frame.features,
      serverTime: frame.serverTime,
    }),
  applyViewUpdate: (frame) =>
    set((s) => ({
      sessions: frame.sessions,
      activeSessionID:
        frame.activeSessionID === undefined ? s.activeSessionID : frame.activeSessionID,
    })),
  selectSession: (id) => set({ activeSessionID: id }),
  setStatus: (status) => set({ status }),
  setDaemonDisconnected: (v) => set({ daemonDisconnected: v }),
  reset: () => set(initialState),
}));
