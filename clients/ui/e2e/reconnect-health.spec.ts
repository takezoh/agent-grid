import {
  type FleetHealthSnapshotFixture,
  installFakeBackend,
  makeSessionInfo,
} from "./support/fake-backend";
import { expect, test } from "./support/test";

const degradedHealth = (): FleetHealthSnapshotFixture => ({
  schemaVersion: 1,
  hostInstanceId: "host-e2e-1",
  revision: 3,
  servers: [
    {
      serverId: "srv-ok",
      serverLabel: "Primary",
      enabled: true,
      observation: { kind: "connected", observedAt: "2026-08-03T00:00:05.000Z" },
    },
    {
      serverId: "srv-down",
      serverLabel: "Build",
      enabled: true,
      observation: {
        kind: "unavailable",
        observedAt: "2026-08-03T00:00:05.000Z",
        reason: "http_error",
      },
    },
    {
      serverId: "srv-off",
      serverLabel: "Legacy",
      enabled: false,
      observation: { kind: "disabled" },
    },
  ],
});

const totalOutageHealth = (): FleetHealthSnapshotFixture => ({
  schemaVersion: 1,
  hostInstanceId: "host-e2e-1",
  revision: 4,
  servers: [
    {
      serverId: "srv-a",
      serverLabel: "Alpha",
      enabled: true,
      observation: {
        kind: "unavailable",
        observedAt: "2026-08-03T00:00:05.000Z",
        reason: "connection_failed",
      },
    },
    {
      serverId: "srv-b",
      serverLabel: "Beta",
      enabled: true,
      observation: {
        kind: "health_unavailable",
        observedAt: "2026-08-03T00:00:06.000Z",
        reason: "refresh_failed",
        prior: { kind: "connected", observedAt: "2026-08-03T00:00:01.000Z" },
      },
    },
  ],
});

const allHealthy = (): FleetHealthSnapshotFixture => ({
  schemaVersion: 1,
  hostInstanceId: "host-e2e-1",
  revision: 5,
  servers: [
    {
      serverId: "srv-a",
      serverLabel: "Alpha",
      enabled: true,
      observation: { kind: "connected", observedAt: "2026-08-03T00:00:10.000Z" },
    },
    {
      serverId: "srv-b",
      serverLabel: "Beta",
      enabled: true,
      observation: { kind: "connected", observedAt: "2026-08-03T00:00:10.000Z" },
    },
  ],
});

function statusBar(page: import("@playwright/test").Page) {
  return page.locator('[data-role="status-bar"]');
}

test("hello-only recovery and independent fleet diagnosis (UAC-001/002/007/008/009)", async ({
  page,
}) => {
  // rejects counterexample: socket-open alone presents recovered/nominal state
  const session = makeSessionInfo({
    id: "recovery-session",
    project: "/repo/app",
    command: "claude",
    title: "Recovery session",
    status: "running",
  });
  const backend = await installFakeBackend(page, {
    sessions: [session],
    health: degradedHealth(),
    hostServers: [
      { serverId: "srv-ok", displayName: "Primary", enabled: true },
      { serverId: "srv-down", displayName: "Build", enabled: true },
      { serverId: "srv-off", displayName: "Legacy", enabled: false },
    ],
    configPath: "/tmp/agent-grid-e2e/servers.json",
    sessionConfig: {
      project_roots: ["/repo/app"],
      project_paths: ["/repo/app"],
      projects: [{ path: "/repo/app", isGit: true }],
      commands: ["claude"],
      push_commands: [],
    },
  });

  await page.goto("/?server=srv-ok#token=test");
  await backend.waitForSocketOpen();

  const connection = statusBar(page).locator('[aria-label="connection state"]');
  // Open without hello must remain non-nominal (UAC-002).
  await expect(connection).not.toHaveAttribute("data-nominal", "true");
  await expect(connection).toContainText(/connecting|reconnecting/i);

  await backend.emit({
    k: "h",
    sessions: [session],
    activeSessionID: session.id,
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  // Active hello clears active recovery while fleet degradation remains (UAC-001/009).
  await expect(connection).toContainText("1/2 servers unavailable");
  await expect(statusBar(page).getByRole("button", { name: "Connections" })).toBeVisible();
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toHaveCount(0);

  await statusBar(page).getByRole("button", { name: "Connections" }).click();
  const panel = page.getByRole("dialog", { name: "Connections" });
  await expect(panel).toBeVisible();
  await expect(panel.locator('[data-server-id="srv-ok"]')).toHaveAttribute(
    "data-state",
    "connected",
  );
  await expect(panel.locator('[data-server-id="srv-down"]')).toHaveAttribute(
    "data-state",
    "unavailable",
  );
  await expect(panel.locator('[data-server-id="srv-off"]')).toHaveAttribute(
    "data-state",
    "disabled",
  );
  await expect(panel.locator('[data-role="config-path"]')).toHaveText(
    "/tmp/agent-grid-e2e/servers.json",
  );
  await expect(panel.getByRole("button", { name: "Copy configuration path" })).toBeVisible();
  // Diagnosis must not own Retry (UAC-008).
  await expect(panel.getByRole("button", { name: "Retry" })).toHaveCount(0);
  // Secret sentinels never appear in assembled DOM.
  await expect(panel).not.toContainText("ticket-test");
  await expect(panel).not.toContainText("Bearer");
});

test("no-session Retry, total outage wording, and healthy clear (UAC-004/010/012/013/014)", async ({
  page,
}) => {
  // Two full exhaustion cycles (auto + manual Retry) can exceed the default 30s budget.
  test.setTimeout(180_000);
  // rejects counterexample: session-only Retry ownership and partial total-outage wording
  const backend = await installFakeBackend(page, {
    sessions: [],
    health: totalOutageHealth(),
    hostServers: [
      { serverId: "srv-a", displayName: "Alpha", enabled: true },
      { serverId: "srv-b", displayName: "Beta", enabled: true },
    ],
    configPath: "/var/lib/agent-grid/servers.json",
  });

  await page.goto("/?server=srv-a#token=test");
  await backend.waitForSocketOpen();
  // Hello with empty sessions proves active recovery without requiring a session list.
  await backend.emit({
    k: "h",
    sessions: [],
    features: ["surface"],
    serverTime: 1_720_000_000_100,
  });

  const connection = statusBar(page).locator('[aria-label="connection state"]');
  // Total outage wording while Connections lists every configured server (UAC-012/013).
  await expect(connection).toContainText("all enabled servers unavailable");
  await statusBar(page).getByRole("button", { name: "Connections" }).click();
  const panel = page.getByRole("dialog", { name: "Connections" });
  await expect(panel.locator('[data-server-id="srv-a"]')).toBeVisible();
  await expect(panel.locator('[data-server-id="srv-b"]')).toBeVisible();
  await expect(panel.locator('[data-server-id="srv-b"]')).toContainText(
    /Health unavailable|stale/i,
  );
  await expect(panel.locator('[data-role="config-path"]')).toHaveText(
    "/var/lib/agent-grid/servers.json",
  );
  await panel.getByRole("button", { name: "Close Connections" }).click();

  // Refuse subsequent gateway sockets (open→close, no hello) so StatusBar exhausts
  // automatic recovery without console 503 noise (UAC-004/010).
  await backend.setRefuseGateway(true);
  const socketsBeforeExhaustion = await backend.gatewaySocketCount();
  await backend.closeGatewaySockets();
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toBeVisible({
    timeout: 90_000,
  });
  await expect(connection).toHaveText("connection closed");
  await statusBar(page).getByRole("button", { name: "Retry" }).focus();
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toBeFocused();
  // Independent axes: fleet diagnosis remains while active is closed.
  await expect(statusBar(page).getByRole("button", { name: "Connections" })).toBeVisible();

  // All-healthy revision clears fleet degradation without clearing Retry (UAC-014).
  await backend.emitHealth(allHealthy());
  await expect(statusBar(page).getByRole("button", { name: "Connections" })).toHaveCount(0);
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toBeVisible();
  await expect(connection).toHaveText("connection closed");

  // Native Retry through the user-visible UI must start a fresh socket lifecycle
  // (component-level Connection.retry() alone is insufficient).
  const socketsAtClosed = await backend.gatewaySocketCount();
  expect(socketsAtClosed).toBeGreaterThan(socketsBeforeExhaustion);
  await statusBar(page).getByRole("button", { name: "Retry" }).click();
  await expect(connection).toContainText(/connecting|reconnecting/i);
  await backend.waitForGatewaySocketCount(socketsAtClosed + 1);

  // Second exhaustion after Retry remains finite and re-exposes Retry (UAC-005/010).
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toBeVisible({
    timeout: 90_000,
  });
  await expect(connection).toHaveText("connection closed");
  // Recovery after the second lifecycle: accept hello on the next Retry socket.
  await backend.setRefuseGateway(false);
  await statusBar(page).getByRole("button", { name: "Retry" }).click();
  await backend.waitForSocketOpen();
  await backend.emit({
    k: "h",
    sessions: [],
    features: ["surface"],
    serverTime: 1_720_000_000_200,
  });
  await expect(connection).not.toHaveText("connection closed");
  await expect(statusBar(page).getByRole("button", { name: "Retry" })).toHaveCount(0);
});
