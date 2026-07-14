import { type Page, expect, test } from "@playwright/test";
import { type FakeBackend, installFakeBackend, makeSessionInfo } from "../support/fake-backend";

const LINE = `${"x".repeat(80)}\n`;

function percentile(samples: number[], p: number): number {
  const sorted = [...samples].sort((a, b) => a - b);
  const idx = Math.min(sorted.length - 1, Math.floor(sorted.length * p));
  return sorted[idx] ?? 0;
}

function makeLargeContent(targetBytes: number): { content: string; lastLine: string } {
  const lineCount = Math.ceil(targetBytes / LINE.length);
  const content = LINE.repeat(lineCount);
  const lastLine = content.slice(Math.max(0, content.length - LINE.length)).trimEnd();
  return { content, lastLine };
}

async function installScrollBench(page: Page): Promise<FakeBackend> {
  const backend = await installFakeBackend(page, {
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: "Large file scroll",
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

  await page.goto("/#token=test");
  await backend.waitForSocketOpen();

  await backend.emit({
    k: "h",
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: "Large file scroll",
        status: "running",
      }),
    ],
    activeSessionID: "session-1",
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  // Changes rows live in the Workspace mode side panel now.
  await page.getByRole("radio", { name: "Workspace" }).click();
  await expect(page.getByTestId("workspace-changes")).toBeVisible();
  return backend;
}

async function benchLargeFileScroll(
  page: Page,
  backend: FakeBackend,
  filePath: string,
  targetBytes: number,
  loadTimeoutMs = 30_000,
): Promise<void> {
  const { content, lastLine } = makeLargeContent(targetBytes);

  await page.route("**/api/sessions/session-1/workspace/file*", async (route) => {
    const url = new URL(route.request().url());
    if (route.request().method() === "GET" && url.searchParams.get("path") === filePath) {
      await route.fulfill({
        status: 200,
        contentType: "application/json",
        body: JSON.stringify({
          path: filePath,
          size: content.length,
          is_binary: false,
          content,
          mtime: "Mon, 01 Jan 2024 00:00:00 GMT",
        }),
      });
      return;
    }
    await route.fulfill({ status: 405, body: "method not allowed" });
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
        path: filePath,
        kind: "read",
        count: 1,
        events: [{ path: filePath, kind: "read" }],
      },
    ],
  });

  await page.locator(`[data-path="${filePath}"]`).click();
  const viewer = page.getByTestId("virtualized-source");
  await expect(viewer).toBeVisible({ timeout: loadTimeoutMs });
  await expect(page.getByText(/\.\.\.\(truncated\)/i)).toHaveCount(0);

  await viewer.focus();

  for (let i = 0; i < 10; i++) {
    await viewer.evaluate((el) => {
      el.scrollTop += 4000;
    });
  }

  const samples: number[] = [];
  for (let i = 0; i < 60; i++) {
    const start = await page.evaluate(() => performance.now());
    await viewer.evaluate((el) => {
      el.scrollTop += 800;
    });
    const end = await page.evaluate(() => performance.now());
    samples.push(end - start);
  }

  const p95 = percentile(samples, 0.95);
  expect(p95).toBeLessThanOrEqual(33);

  await viewer.evaluate((el) => {
    el.scrollTop = el.scrollHeight;
  });
  await expect(viewer.locator("pre")).toContainText(lastLine);
}

test("verify-large-file-scroll-bench: 6 MiB fixture", async ({ page }) => {
  const backend = await installScrollBench(page);
  await benchLargeFileScroll(page, backend, "large-6mib.txt", 6 * 1024 * 1024);
});

// 200 MiB browser scroll is covered by the Vitest harness in
// FileViewer.test.tsx (Chromium cannot JSON-parse + index 200 MiB in-page).
