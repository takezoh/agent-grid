import { installFakeBackend, makeSessionInfo } from "./support/fake-backend";
import { expect, test } from "./support/test";

// Cognitive Workspace Phase 0 (plans/cognitive-work-orchestration-ux.md §27):
// Classic ⇄ Cognitive switching with Classic as the default and one-click
// fallback, the resume-capsule panel with staleness display, and the
// one-action wrong-flag wired to the gateway endpoint.

const SESSION = makeSessionInfo({
  id: "session-1",
  project: "/repo/app",
  command: "claude",
  title: "Login fix",
  status: "running",
});

const CAPSULE = {
  session_id: "session-1",
  project: "/repo/app",
  title: "Login fix",
  objective: "Ship the login fix",
  current_state: "Root cause identified in the token refresh path",
  next_action: "Write the regression test",
  unresolved: ["Flaky CI on windows runners"],
  changed: ["Fixed the token refresh path"],
  verified: ["Unit tests pass"],
  remaining: ["Add the regression test"],
  attention: "Review the fix before release",
  generated_at: "2026-07-05T01:00:00Z",
  source_turns: 4,
};

test("switches Classic to Cognitive, shows capsules, and falls back in one click", async ({
  page,
}) => {
  const backend = await installFakeBackend(page, {
    sessions: [SESSION],
    resumeCapsules: [CAPSULE],
  });

  await page.goto("/?server=test-server#token=test");
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [SESSION],
    activeSessionID: SESSION.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  // Classic is the default workspace.
  const switcher = page.getByTestId("workspace-mode-switch");
  await expect(switcher.getByRole("radio", { name: "Classic" })).toHaveAttribute(
    "aria-checked",
    "true",
  );
  await expect(page.getByTestId("main-with-changes")).toBeVisible();

  // Switch to Cognitive: the capsule panel renders capsule content.
  await switcher.getByRole("radio", { name: "Cognitive" }).click();
  const panel = page.getByRole("region", { name: "cognitive workspace" });
  await expect(panel).toBeVisible();
  await expect(panel.getByText("Ship the login fix")).toBeVisible();
  await expect(panel.getByText("Write the regression test")).toBeVisible();
  await expect(panel.getByText("4 turns")).toBeVisible();

  // Completion report sections (plan §18) render inside the card.
  const completion = panel.getByTestId("capsule-completion");
  await expect(completion.getByText("Fixed the token refresh path")).toBeVisible();
  await expect(completion.getByText("Unit tests pass")).toBeVisible();
  await expect(completion.getByText(/Review the fix before release/)).toBeVisible();

  // One-action wrong-flag posts to the gateway endpoint.
  await panel.getByRole("button", { name: "Mark as wrong" }).click();
  await expect(panel.getByText("Flagged wrong")).toBeVisible();
  expect(backend.capsuleFlagRequests()).toEqual([{ sessionId: "session-1", flagged: true }]);

  // Fallback to Classic is one click and restores the session surface.
  await switcher.getByRole("radio", { name: "Classic" }).click();
  await expect(page.getByTestId("main-with-changes")).toBeVisible();
  await expect(page.locator(".header-bar__title", { hasText: "Login fix" })).toBeVisible();
});

test("marks a capsule stale when the session progressed after generation", async ({ page }) => {
  const backend = await installFakeBackend(page, {
    sessions: [SESSION],
    resumeCapsules: [CAPSULE],
  });

  await page.goto("/?server=test-server#token=test");
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [
      {
        ...SESSION,
        status: {
          lifecycle: "active",
          human_input: false,
          failure: "none",
          changed_at: "2026-07-05T02:00:00Z",
        },
      },
    ],
    activeSessionID: SESSION.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  await page.getByTestId("workspace-mode-switch").getByRole("radio", { name: "Cognitive" }).click();
  const panel = page.getByRole("region", { name: "cognitive workspace" });
  await expect(panel.getByText("Stale")).toBeVisible();
});

test("shows the empty state with the capsule_command hint when no capsules exist", async ({
  page,
}) => {
  const backend = await installFakeBackend(page, {
    sessions: [SESSION],
    resumeCapsules: [],
  });

  await page.goto("/?server=test-server#token=test");
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [SESSION],
    activeSessionID: SESSION.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  await page.getByTestId("workspace-mode-switch").getByRole("radio", { name: "Cognitive" }).click();
  const panel = page.getByRole("region", { name: "cognitive workspace" });
  await expect(panel.getByText("No resume capsules yet.")).toBeVisible();
  await expect(panel.getByText(/capsule_command/)).toBeVisible();
});
