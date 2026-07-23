import { create } from "zustand";
import type { NotificationFrame } from "../wire/server";

export type Notification = {
  id: number;
  level: "info" | "warn" | "error";
  message: string;
  createdAt: number; // epoch ms
  sessionId?: string;
  cmd?: number;
  title?: string;
  body?: string;
};

const MAX_NOTIFICATIONS = 32;

export type NotificationsState = {
  items: Notification[];
  nextId: number;
  /** User mute toggle: while true, incoming notifications are dropped at
      add-time (not queued) so unmuting never replays a stale backlog.
      Persistence lives in NotificationToast (store stays storage-free). */
  muted: boolean;
  add: (n: Omit<Notification, "id" | "createdAt">) => void;
  addFromFrame: (frame: NotificationFrame) => void;
  dismiss: (id: number) => void;
  clear: () => void;
  setMuted: (muted: boolean) => void;
};

export const useNotificationsStore = create<NotificationsState>()((set) => ({
  items: [],
  nextId: 1,
  muted: false,
  add: (n) =>
    set((s) => {
      if (s.muted) return s;
      const next: Notification = {
        id: s.nextId,
        level: n.level,
        message: n.message,
        createdAt: Date.now(),
        sessionId: n.sessionId,
        cmd: n.cmd,
        title: n.title,
        body: n.body,
      };
      const items = [...s.items, next];
      // LRU: drop oldest if over cap
      while (items.length > MAX_NOTIFICATIONS) {
        items.shift();
      }
      return { items, nextId: s.nextId + 1 };
    }),
  addFromFrame: (frame) =>
    set((s) => {
      if (s.muted) return s;
      const message = frame.title ?? frame.body ?? `OSC ${frame.cmd}`;
      const next: Notification = {
        id: s.nextId,
        level: "info",
        message,
        createdAt: frame.nowMs,
        sessionId: frame.sessionId,
        cmd: frame.cmd,
        title: frame.title,
        body: frame.body,
      };
      const items = [...s.items, next];
      // LRU: drop oldest if over cap
      while (items.length > MAX_NOTIFICATIONS) {
        items.shift();
      }
      return { items, nextId: s.nextId + 1 };
    }),
  dismiss: (id) => set((s) => ({ items: s.items.filter((it) => it.id !== id) })),
  clear: () => set({ items: [] }),
  // Muting also clears what is currently on screen so the toggle takes
  // effect immediately instead of waiting out the 5s auto-dismiss.
  setMuted: (muted) => set((s) => (s.muted === muted ? s : { muted, items: muted ? [] : s.items })),
}));
