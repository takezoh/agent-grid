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

  await page.goto("/?server=test-server#token=test");
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
      version: 2,
      profile_id: "claude",
      profile_revision: "claude-e2e",
      parameters: {},
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

test("project and command candidate lists remain scrollable while their filters stay fixed", async ({
  page,
}) => {
  const projects = Array.from({ length: 60 }, (_, index) => ({
    path: `/repo/project-${String(index).padStart(2, "0")}`,
    isGit: true,
    isSandboxed: true,
  }));
  const commands = Array.from(
    { length: 60 },
    (_, index) => `command-${String(index).padStart(2, "0")}`,
  );
  const activeSession = makeSessionInfo({
    id: "session-scroll",
    project: projects[0]?.path ?? "/repo/project-00",
    command: commands[0] ?? "command-00",
    title: "Scroll fixture",
    status: "running",
  });
  const backend = await installFakeBackend(page, {
    sessions: [activeSession],
    sessionConfig: {
      project_roots: projects.map((project) => project.path),
      project_paths: projects.map((project) => project.path),
      projects,
      commands,
      push_commands: [],
    },
  });

  await page.emulateMedia({ reducedMotion: "reduce" });
  await page.setViewportSize({ width: 900, height: 500 });
  await page.goto("/?server=test-server#token=test");
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [activeSession],
    activeSessionID: activeSession.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });
  await page.getByRole("button", { name: "Open command menu" }).click();
  await page.getByTestId("palette-input").press("Enter");

  const projectFilter = page.getByTestId("palette-param-project-filter");
  const projectList = page.locator("#palette-param-project");
  await expect(projectFilter).toBeVisible();
  await expect(projectList).toBeVisible();
  const paramInsets = await page.locator("[data-param-id='project']").evaluate((param) => {
    const body = param.closest(".palette-body");
    if (!(body instanceof HTMLElement)) throw new Error("palette body is missing");
    const bodyRect = body.getBoundingClientRect();
    const paramRect = param.getBoundingClientRect();
    return {
      left: paramRect.left - bodyRect.left,
      right: bodyRect.right - paramRect.right,
    };
  });
  expect(paramInsets.right, `param insets must match: ${JSON.stringify(paramInsets)}`).toBe(
    paramInsets.left,
  );
  const projectMetrics = await projectList.evaluate((element) => {
    const nodes = [
      element,
      element.parentElement,
      element.parentElement?.parentElement,
      element.closest(".palette-param-select"),
      element.closest(".palette-body"),
    ].filter((node): node is HTMLElement => node instanceof HTMLElement);
    return nodes.map((node) => ({
      className: node.className,
      clientHeight: node.clientHeight,
      scrollHeight: node.scrollHeight,
      height: getComputedStyle(node).height,
      flex: getComputedStyle(node).flex,
      minHeight: getComputedStyle(node).minHeight,
      overflowY: getComputedStyle(node).overflowY,
    }));
  });
  expect(
    projectMetrics[0]?.scrollHeight > (projectMetrics[0]?.clientHeight ?? 0),
    `the project list itself must own a constrained scroll viewport: ${JSON.stringify(projectMetrics)}`,
  ).toBe(true);
  const projectFilterY = (await projectFilter.boundingBox())?.y;
  await projectList.hover();
  await page.mouse.wheel(0, 500);
  await expect.poll(() => projectList.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  expect((await projectFilter.boundingBox())?.y).toBe(projectFilterY);

  await projectFilter.press("Enter");
  const commandFilter = page.getByTestId("palette-param-command-filter");
  const commandList = page.locator("#palette-param-command");
  await expect(commandFilter).toBeVisible();
  await expect(commandList).toBeVisible();
  expect(
    await commandList.evaluate((element) => element.scrollHeight > element.clientHeight),
    "the command list itself must own a constrained scroll viewport",
  ).toBe(true);
  const projectSummaryY = (await page.getByTestId("palette-project-summary").boundingBox())?.y;
  const commandFilterY = (await commandFilter.boundingBox())?.y;
  const actionsY = (await page.getByTestId("palette-actions").boundingBox())?.y;
  const worktree = page.getByRole("switch", { name: "worktree" });
  const host = page.getByRole("switch", { name: "Run on host" });
  const worktreeY = (await worktree.boundingBox())?.y;
  const hostY = (await host.boundingBox())?.y;
  await commandList.hover();
  await page.mouse.wheel(0, 500);
  await expect.poll(() => commandList.evaluate((element) => element.scrollTop)).toBeGreaterThan(0);
  expect((await page.getByTestId("palette-project-summary").boundingBox())?.y).toBe(
    projectSummaryY,
  );
  expect((await commandFilter.boundingBox())?.y).toBe(commandFilterY);
  expect((await page.getByTestId("palette-actions").boundingBox())?.y).toBe(actionsY);
  expect((await worktree.boundingBox())?.y).toBe(worktreeY);
  expect((await host.boundingBox())?.y).toBe(hostY);

  const dialog = page.getByRole("dialog", { name: "Command Palette" });
  const dialogBox = await dialog.boundingBox();
  const dialogGeometry = await dialog.evaluate((element) => {
    const sheet = element.parentElement;
    const dialogStyle = getComputedStyle(element);
    const sheetStyle = sheet ? getComputedStyle(sheet) : null;
    return {
      dialogRect: element.getBoundingClientRect().toJSON(),
      dialogClientWidth: element.clientWidth,
      dialogGutter: dialogStyle.scrollbarGutter,
      sheetRect: sheet?.getBoundingClientRect().toJSON(),
      sheetClientWidth: sheet?.clientWidth,
      sheetGutter: sheetStyle?.scrollbarGutter,
    };
  });
  expect(dialogBox).not.toBeNull();
  const viewport = page.viewportSize();
  expect(viewport).not.toBeNull();
  expect(
    Math.abs(
      (dialogBox?.x ?? 0) -
        ((viewport?.width ?? 0) - ((dialogBox?.x ?? 0) + (dialogBox?.width ?? 0))),
    ),
    `the palette dialog must have equal left and right viewport margins: ${JSON.stringify(dialogGeometry)}`,
  ).toBeLessThanOrEqual(1);
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

  await page.goto("/?server=test-server#token=test");
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
    sessions: [
      {
        ...resumed,
        status: { lifecycle: "active", human_input: false, failure: "none" },
      },
    ],
  });
  await expect(page.locator('.run-state-badge[aria-label="status: Working"]')).toBeVisible();

  await backend.emit({
    k: "v",
    sessions: [
      {
        ...resumed,
        status: { lifecycle: "idle", human_input: true, failure: "none" },
      },
    ],
  });
  await expect(
    page.locator('.run-state-badge[aria-label="status: Action required"]'),
  ).toBeVisible();
});
