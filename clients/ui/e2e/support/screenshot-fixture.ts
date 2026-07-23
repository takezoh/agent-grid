import type { Page } from "@playwright/test";
import { installFakeBackend, makeSessionInfo } from "./fake-backend";

const LARGE_FILE_PATH = "src/main.ts";

/** Bootstrap a deterministic session + activity for UI screenshot surveys. */
export async function bootstrapScreenshotApp(page: Page): Promise<void> {
  const backend = await installFakeBackend(page, {
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/home/dev/dev/agent-grid",
        command: "claude",
        title: "Fix WS reconnect backoff",
        status: "running",
      }),
    ],
    sessionConfig: {
      project_roots: ["/home/dev/dev/agent-grid"],
      project_paths: ["/home/dev/dev/agent-grid"],
      projects: [{ path: "/home/dev/dev/agent-grid", isGit: true, isSandboxed: false }],
      commands: ["claude"],
      push_commands: ["save", "status"],
    },
  });

  await page.goto("/#token=test");
  await backend.waitForSocketOpen();

  await backend.emit({
    k: "h",
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/home/dev/dev/agent-grid",
        command: "claude",
        title: "Fix WS reconnect backoff",
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
        kind: "edit",
        count: 3,
        events: [{ path: LARGE_FILE_PATH, kind: "edit" }],
      },
      {
        type: "turn_row",
        session_id: "session-1",
        sequence: 2,
        turn_id: "turn-1",
        path: "README.md",
        kind: "create",
        count: 1,
        events: [{ path: "README.md", kind: "create" }],
      },
    ],
  });
}

/** Apply theme via the same contract as ThemeProvider (dataset + localStorage). */
export async function applyTheme(page: Page, theme: "dark" | "light"): Promise<void> {
  await page.evaluate((t) => {
    document.documentElement.dataset.theme = t;
    localStorage.setItem("agent-grid-theme", t);
  }, theme);
}
