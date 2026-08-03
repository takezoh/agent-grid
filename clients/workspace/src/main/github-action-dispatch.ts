import type {
  Account,
  AccountProjection,
  Catalog,
} from "./github-host.js";
import {
  githubActionResult,
  type GitHubAction,
  type GitHubActionResult,
} from "./repository-lease.js";

type ActionAccount = Pick<
  Account,
  "projection" | "startDeviceFlow" | "cancelDeviceFlow" | "pollDeviceFlow"
>;
type ActionCatalog = Pick<Catalog, "projection" | "refresh">;

export function createGitHubActionDispatcher(
  account: ActionAccount,
  catalog: ActionCatalog,
  statusAccount: () => AccountProjection = () => account.projection(),
): (action: GitHubAction) => Promise<GitHubActionResult> {
  return async (action) => {
    if (action === "status") {
      return githubActionResult(statusAccount(), catalog.projection());
    }
    if (action === "start") {
      let projection: AccountProjection;
      try {
        projection = await account.startDeviceFlow();
      } catch (error) {
        projection = account.projection();
        if (projection.state !== "host_misconfigured") throw error;
      }
      return githubActionResult(projection, catalog.projection());
    }
    if (action === "cancel") {
      return githubActionResult(account.cancelDeviceFlow(), catalog.projection());
    }
    if (action === "poll") {
      const projection = await account.pollDeviceFlow();
      if (projection.state === "authenticated") {
        // Authentication is the latency-critical result. Repository discovery is
        // independent and may span many pages, so never hold the Device Flow IPC
        // response open while the catalog is refreshed.
        void catalog.refresh().catch(() => {
          // Catalog refresh is source-local; retain its current typed projection.
        });
      }
      return githubActionResult(projection, catalog.projection());
    }
    if (action === "refresh_catalog") {
      try {
        return githubActionResult(statusAccount(), await catalog.refresh());
      } catch {
        return githubActionResult(statusAccount(), catalog.projection());
      }
    }
    return githubActionResult(
      { state: "host_misconfigured", failure_code: "host_misconfigured" },
      catalog.projection(),
      "unknown action",
    );
  };
}
