import { expect, test } from "@playwright/test";
import { installFakeBackend, makeSessionInfo } from "../support/fake-backend";

const LARGE_FILE_PATH = "large.txt";
const LINE = `${"x".repeat(80)}\n`;
const LARGE_CONTENT = LINE.repeat(Math.ceil((1024 * 1024) / LINE.length));

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
      projects: [{ path: "/repo/app", isGit: true, isSandboxed: false }],
      commands: ["claude"],
      push_commands: [],
    },
  });

  await page.route("**/api/sessions/session-1/workspace/root-handle", async (route) => {
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

  await page.route(`**/api/sessions/session-1/workspace/file*`, async (route) => {
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

  await page.goto("/#token=test");
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
    activeSessionID: "session-1",
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  await backend.emit({
    k: "v",
    activity_session_id: "session-1",
    activity_events: [
      {
        type: "turn_row",
        session_id: "session-1",
        sequence: 1,
        turn_id: "turn-1",
        path: LARGE_FILE_PATH,
        kind: "read",
        count: 1,
        events: [{ path: LARGE_FILE_PATH, kind: "read" }],
      },
    ],
  });

  await expect(page.getByTestId("activity-rail")).toBeVisible();
  await page.locator(`[data-path="${LARGE_FILE_PATH}"]`).click();
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
  expect(p95).toBeLessThanOrEqual(33);
  expect(p99).toBeLessThanOrEqual(50);
});