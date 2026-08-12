import { installFakeBackend } from "./support/fake-backend";
import {
  SYNTHETIC_GITHUB_USER_CODE,
  SYNTHETIC_GITHUB_VERIFICATION_URI,
  installFakeGitHubHost,
} from "./support/fake-github-host";
import { expect, test } from "./support/test";

const sessionConfig = {
  project_roots: ["/repo"],
  project_paths: ["/repo/a"],
  projects: [{ path: "/repo/a", isGit: true }],
  commands: ["codex"],
  push_commands: [],
};

function repositoryProjection(account: "authenticated" | "unauthenticated" = "authenticated") {
  const authenticated = account === "authenticated";
  return {
    sources: [
      { id: "projects", available: true },
      {
        id: "github",
        available: authenticated,
        status: authenticated ? "complete-current" : "unauthenticated",
      },
    ],
    account: authenticated
      ? { state: "authenticated", login: "alice" }
      : { state: "unauthenticated" },
    catalog: authenticated
      ? {
          outcome: "complete-current",
          generation: 1,
          repositories: [{ repository_id: "10", owner: "alice", name: "demo" }],
        }
      : { outcome: "authentication-required-retained", generation: 0, repositories: [] },
    targets: [
      {
        source: "project",
        project: { server_id: "test-server", project: "/repo/a", server_label: "Test" },
      },
      ...(authenticated
        ? [
            {
              source: "repository",
              repository: { repository_id: "10", owner: "alice", name: "demo" },
            },
          ]
        : []),
    ],
    preparations: [],
  };
}

async function openNewSession(page: import("@playwright/test").Page) {
  await page.goto("/?server=test-server#token=test");
  await page.getByRole("button", { name: "Open command menu" }).click();
  await page.getByTestId("palette-input").press("Enter");
  await expect(page.getByTestId("target-picker")).toBeVisible();
}

test("New Session signs into GitHub and refetches repositories", async ({ page }) => {
  await installFakeBackend(page, { sessions: [], sessionConfig });
  const github = await installFakeGitHubHost(page);
  await openNewSession(page);
  const requestsBefore = github.repositoryRequests;

  await page.getByTestId("github-auth-entry").click();
  await expect(page.getByTestId("github-device-code")).toHaveText(SYNTHETIC_GITHUB_USER_CODE);
  await expect(page.getByTestId("github-device-uri")).toContainText(
    SYNTHETIC_GITHUB_VERIFICATION_URI,
  );
  await page.getByTestId("github-settings-poll").click();

  await expect(page.getByTestId("github-settings-surface")).toHaveCount(0);
  await expect(page.getByText("fixture-user/demo")).toBeVisible({ timeout: 5000 });
  expect(github.actionLog).toEqual(["start", "poll"]);
  expect(github.repositoryRequests).toBeGreaterThan(requestsBefore);
});

test("GitHub failure remains source-scoped and Projects stays usable", async ({ page }) => {
  await installFakeBackend(page, { sessions: [], sessionConfig });
  const github = await installFakeGitHubHost(page, "poll-failure");
  await openNewSession(page);
  await page.getByTestId("github-auth-entry").click();
  await page.getByTestId("github-settings-poll").click();

  await page.getByRole("tab", { name: "Projects" }).click();
  await expect(page.locator('[data-source="project"]').first()).toBeVisible();
  expect(github.actionLog).toEqual(["start", "poll"]);
  await page.getByTestId("palette-param-project-filter").press("Enter");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
});

test("New Session defaults to GitHub repositories and switches source tabs", async ({ page }) => {
  await installFakeBackend(page, { sessions: [], sessionConfig });
  await page.route("**/host/repository", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(repositoryProjection()),
    }),
  );
  await openNewSession(page);

  const githubTab = page.getByRole("tab", { name: "GitHub repositories" });
  const projectsTab = page.getByRole("tab", { name: "Projects" });
  const listbox = page.getByTestId("target-listbox");
  await expect(githubTab).toHaveAttribute("aria-selected", "true");
  await expect(listbox.getByText("alice/demo")).toBeVisible();
  await expect(listbox.locator('[data-source="project"]')).toHaveCount(0);

  await projectsTab.click();
  await expect(projectsTab).toHaveAttribute("aria-selected", "true");
  await expect(listbox.locator('[data-source="project"]').first()).toBeVisible();
  await expect(listbox.locator('[data-source="repository"]')).toHaveCount(0);
});

test("repository uses the shared Execution Server and Command flow with a versioned profile", async ({
  page,
}) => {
  await installFakeBackend(page, { sessions: [], sessionConfig });
  await page.route("**/host/repository", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(repositoryProjection()),
    }),
  );
  const creates: Array<Record<string, unknown>> = [];
  await page.route("**/host/fleet/servers/*/repository-sessions", async (route) => {
    creates.push(route.request().postDataJSON() as Record<string, unknown>);
    await route.fulfill({
      status: 202,
      contentType: "application/json",
      body: JSON.stringify({ preparation_id: "p1", intent_id: "i1" }),
    });
  });

  await openNewSession(page);
  await page.getByText("alice/demo").click();

  await expect(page.getByTestId("execution-server-selector")).toBeVisible();
  await expect(page.getByTestId("execution-server-current")).toHaveText("Test");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
  await expect(page.getByTestId("repository-server-confirm")).toHaveCount(0);
  await page.getByTestId("palette-submit").click();

  await expect.poll(() => creates.length).toBe(1);
  expect(creates[0]).toEqual({
    repository_id: "10",
    owner: "alice",
    name: "demo",
    version: 2,
    profile_id: "codex",
    profile_revision: "codex-e2e",
    parameters: {},
    cols: expect.any(Number),
    rows: expect.any(Number),
  });
  await expect(page.getByRole("dialog", { name: "Command Palette" })).toHaveCount(0);
});

test("unauthenticated GitHub still allows the Project flow", async ({ page }) => {
  await installFakeBackend(page, { sessions: [], sessionConfig });
  await page.route("**/host/repository", (route) =>
    route.fulfill({
      status: 200,
      contentType: "application/json",
      body: JSON.stringify(repositoryProjection("unauthenticated")),
    }),
  );
  await openNewSession(page);
  await expect(page.getByTestId("github-auth-entry")).toBeVisible();
  await page.getByRole("tab", { name: "Projects" }).click();
  await page.getByTestId("palette-param-project-filter").press("Enter");
  await expect(page.getByTestId("palette-param-command-filter")).toBeVisible();
});
