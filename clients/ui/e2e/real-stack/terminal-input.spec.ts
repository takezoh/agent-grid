import path from "node:path";
import type { Page } from "@playwright/test";
import { expect, test } from "../support/test";

// RED / observation (debug-run-workspace-electron-terminal-dead)
//
// status.spec.ts drives the PTY through a hand-rolled `/ws?session=<bare id>`
// socket, so it proves nothing about the frames the SPA itself builds. This
// spec selects a session through the SPA and types into the rendered
// TerminalPane, which is the only path that exercises
// TerminalPane -> Connection.send -> gateway with a fleet-qualified key.
//
// The contract it pins: keystrokes typed into the SPA's terminal reach the PTY
// and echo back. It also reports the terminal subscription phase so a blank
// terminal can be told apart from a live terminal with a dead input path.

const PROJECT_ROOT = path.resolve(process.cwd(), "../..");

type CreatedSession = { id: string };

function serverPath(serverId: string, pathname: string): string {
  return `/host/servers/${encodeURIComponent(serverId)}${pathname}`;
}

function fleetSessionId(serverId: string, sessionId: string): string {
  return JSON.stringify([serverId, sessionId]);
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

test("SPA-typed terminal input reaches the PTY", async ({ page }) => {
  // Record every outbound client frame so the observation can name the exact
  // sessionId the SPA put on the wire, not just the visible consequence.
  await page.addInitScript(() => {
    const NativeWebSocket = globalThis.WebSocket;
    const frames: string[] = [];
    class RecordingWebSocket extends NativeWebSocket {
      send(data: string | ArrayBufferLike | Blob | ArrayBufferView): void {
        if (typeof data === "string") frames.push(data);
        super.send(data);
      }
    }
    Object.defineProperty(globalThis, "WebSocket", {
      configurable: true,
      writable: true,
      value: RecordingWebSocket,
    });
    (globalThis as typeof globalThis & { __sentFrames?: string[] }).__sentFrames = frames;
  });

  const projection = (await (await page.request.get("/host/config")).json()) as {
    servers: Array<{ serverId: string }>;
  };
  const serverId = projection.servers[0].serverId;
  const sessionConfig = (await (
    await page.request.get(serverPath(serverId, "/api/session-config"))
  ).json()) as {
    projects: Array<{ path: string; command_profiles: Array<{ id: string; revision: string }> }>;
  };
  const profileRevision = sessionConfig.projects
    .find((project) => project.path === PROJECT_ROOT)
    ?.command_profiles.find((profile) => profile.id === "codex")?.revision;
  expect(profileRevision).toBeTruthy();

  await page.goto(`/?server=${encodeURIComponent(serverId)}`);
  const sessionId = await createCodexSession(page, serverId, profileRevision as string);
  const key = fleetSessionId(serverId, sessionId);

  const row = page.locator(`.session-list__row[data-session-id=${JSON.stringify(key)}]`);
  await expect(row).toBeVisible();
  await row.click();

  // (a) Is a terminal rendered at all?
  const host = page.locator(".terminal-host");
  await expect(host).toBeVisible();
  await expect(host.locator(".xterm-screen")).toBeVisible();

  // (b) Did the subscription confirm? PTY output can only reach the pane after
  // the `ld` subscribe is accepted and the inbound rewrite matches the pane's
  // filter, so observed output is the end-to-end witness for the output path.
  await expect(host.locator(".xterm-screen")).toContainText("threadId=", { timeout: 15_000 });

  // (c) Does a keystroke typed into the SPA terminal reach the PTY?
  await page.locator(".xterm-helper-textarea").fill("");
  await page.locator(".xterm-helper-textarea").press("h");
  await page.locator(".xterm-helper-textarea").press("i");
  await page.locator(".xterm-helper-textarea").press("Enter");

  const allFrames = await page.evaluate(
    () => (globalThis as typeof globalThis & { __sentFrames?: string[] }).__sentFrames ?? [],
  );
  console.log(`OBSERVED_FRAMES(${allFrames.length}):\n${allFrames.join("\n")}`);
  const inputFrames = allFrames
    .map((raw) => JSON.parse(raw) as { k?: string; sessionId?: string })
    .filter((frame) => frame.k === "i");
  expect(inputFrames.length).toBeGreaterThan(0);
  // Counterexample this rejects: the SPA emitting the compound fleet key, which
  // the gateway cannot resolve and drops silently in WriteRaw. Compare the
  // decoded field — a substring check against the raw JSON text would never
  // match, because the key is re-escaped when the frame is serialised.
  expect(inputFrames.map((frame) => frame.sessionId)).not.toContain(key);

  await expect(host.locator(".xterm-screen")).toContainText("hi", { timeout: 15_000 });
});
