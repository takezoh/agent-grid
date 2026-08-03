import type { Page } from "@playwright/test";

export const SYNTHETIC_GITHUB_USER_CODE = "TEST-CODE";
export const SYNTHETIC_GITHUB_VERIFICATION_URI = "https://oauth.test.invalid/device";

type GitHubFakeMode = "success" | "poll-failure";

export interface FakeGitHubHost {
  readonly actionLog: string[];
  readonly repositoryRequests: number;
}

/**
 * T1 browser-wiring fake for the host-neutral SPA contract. This deliberately
 * intercepts routes and is not evidence for production Web Host route fidelity.
 */
export async function installFakeGitHubHost(
  page: Page,
  mode: GitHubFakeMode = "success",
): Promise<FakeGitHubHost> {
  const actionLog: string[] = [];
  let repositoryRequests = 0;
  let state: "unauthenticated" | "authorizing" | "authenticated" | "failed" = "unauthenticated";

  await page.route("**/host/github", async (route) => {
    const body = route.request().postDataJSON() as { action?: unknown };
    const action = typeof body.action === "string" ? body.action : "invalid";
    actionLog.push(action);

    if (action === "start") state = "authorizing";
    if (action === "poll") state = mode === "success" ? "authenticated" : "failed";

    const result =
      state === "authorizing"
        ? {
            outcome: "authorizing",
            account: {
              state: "authorizing",
              user_code: SYNTHETIC_GITHUB_USER_CODE,
              verification_uri: SYNTHETIC_GITHUB_VERIFICATION_URI,
            },
            catalog: emptyCatalog(),
          }
        : state === "authenticated"
          ? {
              outcome: "success",
              account: { state: "authenticated", login: "fixture-user" },
              catalog: authenticatedCatalog(),
            }
          : state === "failed"
            ? {
                outcome: "failure",
                account: { state: "refresh_failed", failure_code: "fixture_failure" },
                catalog: emptyCatalog(),
              }
            : {
                outcome: "unauthenticated",
                account: { state: "unauthenticated" },
                catalog: emptyCatalog(),
              };

    await route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: { "Cache-Control": "no-store" },
      body: JSON.stringify(result),
    });
  });

  await page.route("**/host/repository", async (route) => {
    repositoryRequests += 1;
    const authenticated = state === "authenticated";
    await route.fulfill({
      status: 200,
      contentType: "application/json",
      headers: { "Cache-Control": "no-store" },
      body: JSON.stringify({
        sources: [
          { id: "projects", available: true },
          {
            id: "github",
            available: authenticated,
            status: authenticated ? "complete-current" : state,
          },
        ],
        account: authenticated
          ? { state: "authenticated", login: "fixture-user" }
          : { state: state === "failed" ? "refresh_failed" : state },
        catalog: authenticated ? authenticatedCatalog() : emptyCatalog(),
        targets: [
          {
            source: "project",
            project: { server_id: "test-server", project: "/repo/a", server_label: "Test" },
          },
          ...(authenticated
            ? [
                {
                  source: "repository",
                  repository: {
                    repository_id: "fixture-repo",
                    owner: "fixture-user",
                    name: "demo",
                  },
                },
              ]
            : []),
        ],
        preparations: [],
      }),
    });
  });

  return {
    actionLog,
    get repositoryRequests() {
      return repositoryRequests;
    },
  };
}

function emptyCatalog() {
  return { outcome: "empty", generation: 0, stale: false, repositories: [] };
}

function authenticatedCatalog() {
  return {
    outcome: "complete-current",
    generation: 1,
    stale: false,
    repositories: [{ repository_id: "fixture-repo", owner: "fixture-user", name: "demo" }],
  };
}
