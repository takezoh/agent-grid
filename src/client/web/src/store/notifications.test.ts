import { beforeEach, describe, expect, it } from "vitest";
import { useNotificationsStore } from "./notifications";

describe("notificationsStore", () => {
  beforeEach(() => {
    useNotificationsStore.getState().clear();
  });

  it("add appends an item with a fresh id", () => {
    useNotificationsStore.getState().add({ level: "info", message: "hi" });
    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(1);
    expect(items[0]?.level).toBe("info");
    expect(items[0]?.message).toBe("hi");
  });

  it("dismiss removes by id", () => {
    useNotificationsStore.getState().add({ level: "warn", message: "x" });
    const id = useNotificationsStore.getState().items[0]?.id;
    if (id === undefined) throw new Error("expected id");
    useNotificationsStore.getState().dismiss(id);
    expect(useNotificationsStore.getState().items).toHaveLength(0);
  });

  it("LRU evicts oldest when exceeding 32 items", () => {
    for (let i = 0; i < 40; i++) {
      useNotificationsStore.getState().add({ level: "info", message: `m${i}` });
    }
    const items = useNotificationsStore.getState().items;
    expect(items).toHaveLength(32);
    // oldest preserved should be m8 (40 - 32 = 8 dropped)
    expect(items[0]?.message).toBe("m8");
    expect(items[items.length - 1]?.message).toBe("m39");
  });
});
