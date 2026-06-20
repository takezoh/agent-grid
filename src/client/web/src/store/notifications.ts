import { create } from "zustand";

export type Notification = {
  id: number;
  level: "info" | "warn" | "error";
  message: string;
  createdAt: number; // epoch ms
};

const MAX_NOTIFICATIONS = 32;

export type NotificationsState = {
  items: Notification[];
  nextId: number;
  add: (n: Omit<Notification, "id" | "createdAt">) => void;
  dismiss: (id: number) => void;
  clear: () => void;
};

export const useNotificationsStore = create<NotificationsState>()((set) => ({
  items: [],
  nextId: 1,
  add: (n) =>
    set((s) => {
      const next: Notification = {
        id: s.nextId,
        level: n.level,
        message: n.message,
        createdAt: Date.now(),
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
}));
