import path from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "../support/test";

type CreatedSession = { id: string };
const PROJECT_ROOT = path.resolve(process.cwd(), "../..");

function serverPath(serverId: string, pathname: string): string {
  return `/host/servers/${encodeURIComponent(serverId)}${pathname}`;
}

async function createCodexSession(
  page: Page,
  serverId: string,
  profileRevision: string,
): Promise<string> {
  const response = await page.request.post(serverPath(serverId, "/api/sessions"), {
    data: {
      version: 2,
      project: PROJECT_ROOT,
      profile_id: "codex",
      profile_revision: profileRevision,
      parameters: {},
      cols: 100,
      rows: 30,
      sandbox: "host",
    },
  });
  const body = await response.text();
  expect(response.status(), body).toBe(201);
  return (JSON.parse(body) as CreatedSession).id;
}

async function sendTerminalInput(
  page: Page,
  serverId: string,
  sessionId: string,
  input: string,
): Promise<void> {
  await page.evaluate(
    async ({ server, id, data }) => {
      const prefix = `/host/servers/${encodeURIComponent(server)}`;
      const ticketResponse = await fetch(`${prefix}/api/ws-ticket`, { method: "POST" });
      if (!ticketResponse.ok) throw new Error(`ticket failed: ${ticketResponse.status}`);
      const { ticket } = (await ticketResponse.json()) as { ticket: string };
      const scheme = location.protocol === "https:" ? "wss:" : "ws:";
      const socket = new WebSocket(
        `${scheme}//${location.host}${prefix}/ws?ticket=${encodeURIComponent(ticket)}&session=${encodeURIComponent(id)}`,
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
    { server: serverId, id: sessionId, data: input },
  );
}

function fleetSessionId(serverId: string, sessionId: string): string {
  // SPA session list keys are server-qualified fleet refs.
  return JSON.stringify([serverId, sessionId]);
}

function statusSlot(page: Page, serverId: string, sessionId: string) {
  return page.locator(
    `[data-session-id=${JSON.stringify(fleetSessionId(serverId, sessionId))}] .session-status-slot`,
  );
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
  const projection = (await (await page.request.get("/host/config")).json()) as {
    servers: Array<{ serverId: string }>;
  };
  const serverId = projection.servers[0].serverId;
  const sessionConfig = (await (
    await page.request.get(serverPath(serverId, "/api/session-config"))
  ).json()) as {
    projects: Array<{
      path: string;
      command_profiles: Array<{ id: string; revision: string }>;
    }>;
  };
  const profileRevision = sessionConfig.projects
    .find((project) => project.path === PROJECT_ROOT)
    ?.command_profiles.find((profile) => profile.id === "codex")?.revision;
  expect(profileRevision).toBeTruthy();
  await page.goto(`/?server=${encodeURIComponent(serverId)}`);

  const completedID = await createCodexSession(page, serverId, profileRevision as string);
  const activeID = await createCodexSession(page, serverId, profileRevision as string);
  const approvalID = await createCodexSession(page, serverId, profileRevision as string);
  const completedKey = fleetSessionId(serverId, completedID);
  const activeKey = fleetSessionId(serverId, activeID);
  const approvalKey = fleetSessionId(serverId, approvalID);

  await expect(page.locator(`[data-session-id=${JSON.stringify(completedKey)}]`)).toBeVisible();
  await expect(page.locator(`[data-session-id=${JSON.stringify(activeKey)}]`)).toBeVisible();
  await expect(page.locator(`[data-session-id=${JSON.stringify(approvalKey)}]`)).toBeVisible();

  await sendTerminalInput(page, serverId, completedID, "complete normally");
  await expect(statusSlot(page, serverId, completedID)).toHaveAttribute(
    "aria-label",
    "status: Idle; Notification",
  );
  await expect(statusSlot(page, serverId, completedID).locator(".status-icon__signal")).toHaveText(
    "🔔",
  );
  await expect(statusSlot(page, serverId, completedID).locator(".status-icon__action")).toHaveCount(
    0,
  );
  await expect(
    statusSlot(page, serverId, completedID).locator(".session-status-spinner"),
  ).toHaveCount(0);

  await sendTerminalInput(page, serverId, activeID, "[agent-grid-e2e:active]");
  await expect(statusSlot(page, serverId, activeID)).toHaveAttribute(
    "aria-label",
    "status: Working",
  );
  await expect(
    statusSlot(page, serverId, activeID).locator(".session-status-spinner"),
  ).toBeVisible();
  await expect(statusSlot(page, serverId, activeID).locator(".status-icon__signal")).toHaveCount(0);
  await expect(statusSlot(page, serverId, activeID).locator(".status-icon__action")).toHaveCount(0);

  await sendTerminalInput(page, serverId, approvalID, "[agent-grid-e2e:approval]");
  await expect(statusSlot(page, serverId, approvalID)).toHaveAttribute(
    "aria-label",
    "status: Action required; Notification",
  );
  await expect(statusSlot(page, serverId, approvalID).locator(".status-icon__action")).toHaveText(
    "!",
  );
  await expect(statusSlot(page, serverId, approvalID).locator(".status-icon__signal")).toHaveText(
    "🔔",
  );
  await expect(
    statusSlot(page, serverId, approvalID).locator(".session-status-spinner"),
  ).toHaveCount(0);

  // Reconnect hello carries the same approval signal. The client consumes each
  // epoch/sequence once across connections (fleet membership stays intact), so
  // durable human input remains as "!" while the replayed bell is suppressed.
  await page.evaluate(() => {
    const sockets = (globalThis as typeof globalThis & { __statusE2ETrackedSockets?: WebSocket[] })
      .__statusE2ETrackedSockets;
    const lifecycle = sockets?.find((socket) => {
      const url = new URL(socket.url);
      return url.pathname.endsWith("/ws") && !url.searchParams.has("session");
    });
    if (!lifecycle) throw new Error("lifecycle websocket not found");
    lifecycle.close();
  });
  await expect(statusSlot(page, serverId, approvalID)).toHaveAttribute(
    "aria-label",
    "status: Action required",
  );
  await expect(statusSlot(page, serverId, approvalID).locator(".status-icon__action")).toHaveText(
    "!",
  );
  await expect(statusSlot(page, serverId, approvalID).locator(".status-icon__signal")).toHaveCount(
    0,
  );
  await expect(
    statusSlot(page, serverId, approvalID).locator(".session-status-spinner"),
  ).toHaveCount(0);
});
