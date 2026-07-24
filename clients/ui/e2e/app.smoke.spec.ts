import { installFakeBackend, makeSessionInfo } from "./support/fake-backend";
import { expect, test } from "./support/test";

test("renders live sessions and completes a new-session submission against the fake gateway", async ({
  page,
}) => {
  const backend = await installFakeBackend(page, {
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: "Existing session",
        status: "running",
      }),
    ],
    sessionConfig: {
      project_roots: ["/repo/app", "/repo/tools"],
      project_paths: ["/repo/app", "/repo/tools"],
      projects: [
        { path: "/repo/app", isGit: true, isSandboxed: false },
        { path: "/repo/tools", isGit: false, isSandboxed: true },
      ],
      commands: ["claude", "gemini"],
      push_commands: ["save", "status"],
    },
    createdSessionId: "session-new",
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
        title: "Existing session",
        status: "running",
      }),
    ],
    activeSessionID: "session-1",
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  await expect(page.locator(".header-bar__title", { hasText: "Existing session" })).toBeVisible();
  await expect
    .poll(async () => {
      const frames = await backend.sentFrames();
      return frames.filter((frame) => frame.k === "ld").at(-1);
    })
    .toMatchObject({
      sessionId: "session-1",
      cols: expect.any(Number),
      rows: expect.any(Number),
    });

  await page.getByRole("button", { name: "Open command menu" }).click();
  await expect(page.getByRole("dialog", { name: "Command Palette" })).toBeVisible();

  await page.getByTestId("palette-input").press("Enter");
  await expect(page.getByTestId("palette-param-project-filter")).toBeVisible();
  await page.getByTestId("palette-param-project-filter").press("Enter");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
  // Enter on the final field moves focus to the explicit confirm button —
  // selection alone must NOT create the session.
  await page.getByTestId("palette-param-command-filter").press("Enter");
  expect(await backend.createSessionRequests()).toHaveLength(0);
  await expect(page.getByTestId("palette-submit")).toBeFocused();
  await page.getByTestId("palette-submit").press("Enter");

  await expect.poll(async () => backend.createSessionRequests()).toHaveLength(1);
  const fittedSize = (await backend.sentFrames()).filter((frame) => frame.k === "r").at(-1);
  expect(fittedSize).toMatchObject({ k: "r" });
  const fittedCols = fittedSize?.cols;
  const fittedRows = fittedSize?.rows;
  expect(typeof fittedCols).toBe("number");
  expect(typeof fittedRows).toBe("number");
  expect(await backend.createSessionRequests()).toEqual([
    {
      project: "/repo/app",
      command: "claude",
      cols: fittedCols,
      rows: fittedRows,
    },
  ]);
  await expect
    .poll(async () => {
      const frames = await backend.emittedFrames();
      return frames.filter((frame) => frame.k === "v").at(-1);
    })
    .toMatchObject({
      k: "v",
      sessions: [
        expect.objectContaining({ id: "session-1" }),
        expect.objectContaining({ id: "session-new" }),
      ],
    });
  const latestViewUpdate = (await backend.emittedFrames())
    .filter((frame) => frame.k === "v")
    .at(-1);
  expect(latestViewUpdate).toBeDefined();
  expect(latestViewUpdate).not.toHaveProperty("activeSessionID");

  await expect(page.locator(".header-bar__title", { hasText: "Browser smoke" })).toBeVisible();
  await expect
    .poll(async () => {
      const frames = await backend.sentFrames();
      return frames.filter((frame) => frame.k === "ld").at(-1);
    })
    .toMatchObject({
      sessionId: "session-new",
      cols: expect.any(Number),
      rows: expect.any(Number),
    });
});

test("renders resumed Codex status updates from running to waiting", async ({ page }) => {
  const resumed = makeSessionInfo({
    id: "codex-resumed",
    project: "/repo/app",
    command: "codex",
    title: "Resumed Codex",
    status: "idle",
  });
  const backend = await installFakeBackend(page, {
    sessions: [resumed],
    sessionConfig: {
      project_roots: ["/repo/app"],
      project_paths: ["/repo/app"],
      projects: [{ path: "/repo/app", isGit: true, isSandboxed: false }],
      commands: ["codex"],
      push_commands: [],
    },
  });

  await page.goto("/#token=test");
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [resumed],
    activeSessionID: resumed.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  await backend.emit({
    k: "v",
    sessions: [{ ...resumed, view: { ...resumed.view, status: "running" } }],
  });
  await expect(page.locator('.run-state-badge[aria-label="status: running"]')).toBeVisible();

  await backend.emit({
    k: "v",
    sessions: [{ ...resumed, view: { ...resumed.view, status: "waiting" } }],
  });
  await expect(page.locator('.run-state-badge[aria-label="status: waiting"]')).toBeVisible();
});
