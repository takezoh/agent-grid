import { installFakeBackend, makeSessionInfo } from "../support/fake-backend";
import { expect, test } from "../support/test";

const LARGE_FILE_PATH = "large.txt";
const LINE = `${"x".repeat(80)}\n`;
const LARGE_CONTENT = LINE.repeat(Math.ceil((1024 * 1024) / LINE.length));
// FleetRuntime projects server-qualified session keys into the SPA.
// Prefer the fleet-qualified key; also emit under the bare id because hello
// seed and fleet subscribe can race for which identity is scoped.
const FLEET_SESSION_ID = JSON.stringify(["test-server", "session-1"]);
const ACTIVITY_SESSION_IDS = [FLEET_SESSION_ID, "session-1"] as const;

function percentile(samples: number[], p: number): number {
  const sorted = [...samples].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor(sorted.length * p));
  return sorted[idx] ?? 0;
}

test("verify-editor-large-file-editing-performance: 200 keystrokes stay within budget", async ({
  page,
}) => {
  const backend = await installFakeBackend(page, {
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: "Large file bench",
        status: "running",
      }),
    ],
    sessionConfig: {
      project_roots: ["/repo/app"],
      project_paths: ["/repo/app"],
      projects: [{ path: "/repo/app", isGit: true }],
      commands: ["claude"],
      push_commands: [],
    },
  });

  await page.route("**/workspace/root-handle**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        session_id: "session-1",
        frame_generation: 1,
        resolved_root_path: "/repo/app",
      }),
    });
  });

  await page.route("**/workspace/file**", async (route) => {
    const url = new URL(route.request().url());
    if (route.request().method() === "GET" && url.searchParams.get("path") === LARGE_FILE_PATH) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          path: LARGE_FILE_PATH,
          size: LARGE_CONTENT.length,
          is_binary: false,
          content: LARGE_CONTENT,
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }),
      });
      return;
    }
    await route.fulfill({ status: 405, body: "method not allowed" });
  });

  await page.route("**/workspace/tree**", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({ outcome: "ok", path: "", entries: [] }),
    });
  });

  await page.goto("/?server=test-server#token=test");
  await backend.waitForSocketOpen();

  await backend.emit({
    k: "h",
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: "Large file bench",
        status: "running",
      }),
    ],
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  const sessionOption = page.getByRole("option", { name: /Large file bench/ });
  await expect(sessionOption).toBeVisible();
  await sessionOption.click();

  // Changes rows live in the Workspace mode side panel now.
  await page.getByRole("radio", { name: "Workspace" }).click();
  await expect(page.getByTestId("workspace-changes")).toBeVisible();
  // Emit under both bare and fleet-qualified keys; ChangesRows keys by activeSessionID.
  for (const sessionId of ACTIVITY_SESSION_IDS) {
    await backend.emit({
      k: "v",
      activity_session_id: sessionId,
      activity_events: [
        {
          type: "turn_row",
          session_id: sessionId,
          sequence: 1,
          turn_id: "turn-1",
          path: LARGE_FILE_PATH,
          kind: "edit",
          count: 1,
          events: [{ path: LARGE_FILE_PATH, kind: "read" }],
        },
      ],
    });
  }
  await expect(page.locator(`[data-path="${LARGE_FILE_PATH}"]`)).toBeVisible({ timeout: 10_000 });
  await page.locator(`[data-path="${LARGE_FILE_PATH}"]`).click();
  // Edit rows open the Diff tab by default; switch to Viewer for the editor.
  await page.getByRole("tab", { name: "Viewer" }).click();
  await expect(page.getByTestId("codemirror-editor")).toBeVisible({ timeout: 15_000 });

  const editor = page.getByTestId("codemirror-editor").locator(".cm-content");
  await editor.click();

  // Warm up layout/measurement so the timed burst is steady-state.
  for (let i = 0; i < 20; i++) {
    await editor.press("j");
  }

  const samples: number[] = [];
  for (let i = 0; i < 200; i++) {
    const start = await page.evaluate(() => performance.now());
    await editor.press("j", { delay: 0 });
    const end = await page.evaluate(() => performance.now());
    samples.push(end - start);
  }

  const p95 = percentile(samples, 0.95);
  const p99 = percentile(samples, 0.99);
  // 50ms budget: contested CI agents routinely land p95 in the 40–45ms band
  // while remaining well under a 20fps floor.
  expect(p95).toBeLessThanOrEqual(50);
  expect(p99).toBeLessThanOrEqual(75);
});
