import path from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "../support/test";

type CreatedSession = { id: string };
const PROJECT_ROOT = path.resolve(process.cwd(), "../..");

async function createCodexSession(page: Page): Promise<string> {
  const response = await page.request.post("/api/sessions", {
    data: {
      project: PROJECT_ROOT,
      command: "codex",
      cols: 100,
      rows: 30,
      sandbox: "host",
    },
  });
  expect(response.status()).toBe(201);
  return ((await response.json()) as CreatedSession).id;
}

async function sendTerminalInput(page: Page, sessionId: string, input: string): Promise<void> {
  await page.evaluate(
    async ({ id, data }) => {
      const ticketResponse = await fetch("/api/ws-ticket", { method: "POST" });
      if (!ticketResponse.ok) throw new Error(`ticket failed: ${ticketResponse.status}`);
      const { ticket } = (await ticketResponse.json()) as { ticket: string };
      const scheme = location.protocol === "https:" ? "wss:" : "ws:";
      const socket = new WebSocket(
        `${scheme}//${location.host}/ws?ticket=${encodeURIComponent(ticket)}&session=${encodeURIComponent(id)}`,
      );
      await new Promise<void>((resolve, reject) => {
        socket.onerror = () => reject(new Error("surface websocket failed"));
        socket.onopen = () => {
          socket.send(JSON.stringify({ k: "r", cols: 100, rows: 30 }));
          socket.send(JSON.stringify({ k: "i", d: `${data}\n` }));
          resolve();
        };
      });
      // Keep the surface alive while the app-server emits its lifecycle.
      (
        globalThis as typeof globalThis & { __statusE2ESockets?: WebSocket[] }
      ).__statusE2ESockets ??= [];
      (
        globalThis as typeof globalThis & { __statusE2ESockets: WebSocket[] }
      ).__statusE2ESockets.push(socket);
    },
    { id: sessionId, data: input },
  );
}

function statusSlot(page: Page, sessionId: string) {
  return page.locator(`[data-session-id="${sessionId}"] .session-status-slot`);
}

test("real server and uihost render Codex composite status patterns", async ({ page }) => {
  await page.addInitScript(() => {
    const NativeWebSocket = globalThis.WebSocket;
    const sockets: WebSocket[] = [];
    class TrackedWebSocket extends NativeWebSocket {
      constructor(url: string | URL, protocols?: string | string[]) {
        super(url, protocols);
        sockets.push(this);
      }
    }
    Object.defineProperty(globalThis, "WebSocket", {
      configurable: true,
      writable: true,
      value: TrackedWebSocket,
    });
    (
      globalThis as typeof globalThis & { __statusE2ETrackedSockets?: WebSocket[] }
    ).__statusE2ETrackedSockets = sockets;
  });
  await page.goto("/#token=e2e");

  const completedID = await createCodexSession(page);
  const activeID = await createCodexSession(page);
  const approvalID = await createCodexSession(page);

  await expect(page.locator(`[data-session-id="${completedID}"]`)).toBeVisible();
  await expect(page.locator(`[data-session-id="${activeID}"]`)).toBeVisible();
  await expect(page.locator(`[data-session-id="${approvalID}"]`)).toBeVisible();

  await sendTerminalInput(page, completedID, "complete normally");
  await expect(statusSlot(page, completedID)).toHaveAttribute(
    "aria-label",
    "status: Idle; Notification",
  );
  await expect(statusSlot(page, completedID).locator(".status-icon__signal")).toHaveText("🔔");
  await expect(statusSlot(page, completedID).locator(".status-icon__action")).toHaveCount(0);
  await expect(statusSlot(page, completedID).locator(".session-status-spinner")).toHaveCount(0);

  await sendTerminalInput(page, activeID, "[agent-grid-e2e:active]");
  await expect(statusSlot(page, activeID)).toHaveAttribute("aria-label", "status: Working");
  await expect(statusSlot(page, activeID).locator(".session-status-spinner")).toBeVisible();
  await expect(statusSlot(page, activeID).locator(".status-icon__signal")).toHaveCount(0);
  await expect(statusSlot(page, activeID).locator(".status-icon__action")).toHaveCount(0);

  await sendTerminalInput(page, approvalID, "[agent-grid-e2e:approval]");
  await expect(statusSlot(page, approvalID)).toHaveAttribute(
    "aria-label",
    "status: Action required; Notification",
  );
  await expect(statusSlot(page, approvalID).locator(".status-icon__action")).toHaveText("!");
  await expect(statusSlot(page, approvalID).locator(".status-icon__signal")).toHaveText("🔔");
  await expect(statusSlot(page, approvalID).locator(".session-status-spinner")).toHaveCount(0);

  // Reconnect hello carries the same approval signal. The client consumes each
  // epoch/sequence once across connections, so durable human input remains as
  // "!" while the replayed bell is suppressed.
  await page.evaluate(() => {
    const sockets = (globalThis as typeof globalThis & { __statusE2ETrackedSockets?: WebSocket[] })
      .__statusE2ETrackedSockets;
    const lifecycle = sockets?.find((socket) => {
      const url = new URL(socket.url);
      return url.pathname === "/ws" && !url.searchParams.has("session");
    });
    if (!lifecycle) throw new Error("lifecycle websocket not found");
    lifecycle.close();
  });
  await expect(statusSlot(page, approvalID)).toHaveAttribute(
    "aria-label",
    "status: Action required",
  );
  await expect(statusSlot(page, approvalID).locator(".status-icon__action")).toHaveText("!");
  await expect(statusSlot(page, approvalID).locator(".status-icon__signal")).toHaveCount(0);
  await expect(statusSlot(page, approvalID).locator(".session-status-spinner")).toHaveCount(0);
});
