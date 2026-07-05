import { expect, test } from "@playwright/test";
import { installFakeBackend, makeSessionInfo } from "./support/fake-backend";

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

  await expect(page.getByRole("heading", { name: "Existing session" })).toBeVisible();
  await expect
    .poll(async () => {
      const frames = await backend.sentFrames();
      return frames.filter((frame) => frame.k === "s").map((frame) => frame.sessionId);
    })
    .toContain("session-1");

  await page.getByRole("button", { name: "Open command menu" }).click();
  await expect(page.getByRole("dialog", { name: "Command Palette" })).toBeVisible();

  await page.getByTestId("palette-input").press("Enter");
  await expect(page.getByTestId("palette-param-project-filter")).toBeVisible();
  await page.getByTestId("palette-param-project-filter").press("Enter");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
  await page.getByTestId("palette-param-command-filter").press("Enter");

  await expect.poll(async () => backend.createSessionRequests()).toHaveLength(1);
  expect(await backend.createSessionRequests()).toEqual([
    { project: "/repo/app", command: "claude" },
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

  await expect(page.getByRole("heading", { name: "Browser smoke" })).toBeVisible();
  await expect
    .poll(async () => {
      const frames = await backend.sentFrames();
      return frames.filter((frame) => frame.k === "s").map((frame) => frame.sessionId);
    })
    .toContain("session-new");
});
