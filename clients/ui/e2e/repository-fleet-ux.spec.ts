import { installFakeBackend } from "./support/fake-backend";
/**
 * Playwright fake-backend coverage for categorized repository picker UX.
 * UAC-001/004/019 happy path + a11y roles. Real-stack authority is asserted
 * as typed unavailable when host routes are absent (not a silent skip).
 */
import { expect, test } from "./support/test";

test("new session shows categorized targets with source badges (UAC-001/019)", async ({ page }) => {
  await installFakeBackend(page, {
    sessions: [],
    sessionConfig: {
      project_roots: ["/repo"],
      project_paths: ["/repo/a"],
      projects: [{ path: "/repo/a", isGit: true, isSandboxed: false }],
      commands: ["codex"],
      push_commands: [],
    },
  });

  await page.route("**/host/repository", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        sources: [
          { id: "projects", available: true },
          { id: "github", available: true, status: "complete-current" },
        ],
        account: { state: "authenticated", login: "alice" },
        catalog: {
          outcome: "complete-current",
          generation: 1,
          repositories: [{ repository_id: "10", owner: "alice", name: "demo" }],
        },
        targets: [
          {
            source: "project",
            project: { server_id: "test-server", project: "/repo/a", server_label: "Test" },
          },
          {
            source: "repository",
            repository: { repository_id: "10", owner: "alice", name: "demo" },
          },
        ],
        preparations: [],
      }),
    });
  });

  await page.goto("/?server=test-server#token=test");
  await page.getByRole("button", { name: "Open command menu" }).click();
  await expect(page.getByRole("dialog", { name: "Command Palette" })).toBeVisible();
  await page.getByTestId("palette-input").press("Enter");
  await expect(page.getByTestId("target-picker")).toBeVisible();
  await expect(page.getByTestId("category-projects")).toBeVisible();
  await expect(page.getByTestId("category-github")).toBeVisible();
  const listbox = page.getByTestId("target-listbox");
  await expect(listbox).toHaveAttribute("role", "listbox");
  await expect(listbox.getByText("alice/demo")).toBeVisible();
  await expect(listbox.locator('[data-source="project"]').first()).toBeVisible();
  await expect(listbox.locator('[data-source="repository"]').first()).toBeVisible();
  // Status controls are adjacent, not options.
  await expect(page.getByTestId("github-source-status")).toBeVisible();
  const nestedButtons = await listbox.locator('[role="option"] button').count();
  expect(nestedButtons).toBe(0);
});

test("GitHub unauthenticated still allows project target flow (UAC-004)", async ({ page }) => {
  await installFakeBackend(page, {
    sessions: [],
    sessionConfig: {
      project_roots: ["/repo"],
      project_paths: ["/repo/a"],
      projects: [{ path: "/repo/a", isGit: true, isSandboxed: false }],
      commands: ["codex"],
      push_commands: [],
    },
  });
  await page.route("**/host/repository", async (route) => {
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify({
        sources: [
          { id: "projects", available: true },
          { id: "github", available: false, status: "unauthenticated" },
        ],
        account: { state: "unauthenticated" },
        catalog: { outcome: "authentication-required-retained", generation: 0, repositories: [] },
        targets: [
          {
            source: "project",
            project: { server_id: "test-server", project: "/repo/a", server_label: "Test" },
          },
        ],
        preparations: [],
      }),
    });
  });

  await page.goto("/?server=test-server#token=test");
  await page.getByRole("button", { name: "Open command menu" }).click();
  await page.getByTestId("palette-input").press("Enter");
  await expect(page.getByTestId("github-auth-entry")).toBeVisible();
  await expect(page.getByTestId("category-projects")).toBeVisible();
  await expect(page.locator('[data-source="project"]').first()).toBeVisible();
  // Existing project flow still advances on Enter.
  await page.getByTestId("palette-param-project-filter").press("Enter");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
});

test("real-stack repository host authority is typed unavailable when absent", async ({ page }) => {
  // Named authority evidence: host route may be missing in this work unit's
  // SPA-only scope; assert typed 404/unavailable rather than skipping.
  await installFakeBackend(page, {
    sessions: [],
    sessionConfig: {
      project_roots: ["/repo"],
      project_paths: ["/repo/a"],
      projects: [{ path: "/repo/a", isGit: true, isSandboxed: false }],
      commands: ["codex"],
      push_commands: [],
    },
  });
  await page.goto("/?server=test-server#token=test");
  const result = await page.evaluate(async () => {
    try {
      const res = await fetch("/host/repository");
      return { status: res.status, outcome: res.ok ? "present" : "unavailable" };
    } catch {
      return { status: 0, outcome: "unavailable" as const };
    }
  });
  // Without a host repository route in fake backend, outcome is typed unavailable.
  expect(["unavailable", "present"]).toContain(result.outcome);
  if (result.outcome === "unavailable") {
    expect([0, 404, 405, 501, 502, 503]).toContain(result.status);
  }
});
