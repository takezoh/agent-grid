import { expect, test } from "../support/test";

/**
 * Real-stack cross-boundary traces for resilient reconnect + fleet health.
 * Observes assembled host→SPA delivery against real server + web-host.
 */

test("real stack recovers active connection on hello and exposes fleet health without secrets", async ({
  page,
}) => {
  // rejects counterexample: delivery of raw tickets/tokens into StatusBar/DOM
  const projection = (await (await page.request.get("/host/config")).json()) as {
    servers: Array<{ serverId: string }>;
  };
  expect(projection.servers.length).toBeGreaterThan(0);
  const serverId = projection.servers[0].serverId;
  await page.goto(`/?server=${encodeURIComponent(serverId)}`);

  const status = page.locator('[data-role="status-bar"] [aria-label="connection state"]');
  await expect(status).toBeVisible({ timeout: 30_000 });
  // Real host eventually delivers hello-proved open (or fleet-checking then open).
  await expect
    .poll(async () => status.getAttribute("data-nominal"), { timeout: 60_000 })
    .toBe("true");

  // Single healthy configured member: Connections remains hidden once healthy.
  await expect(page.getByRole("button", { name: "Connections" })).toHaveCount(0, {
    timeout: 30_000,
  });

  // Credential-bearing material never surfaces in the status shell.
  const shell = page.locator("body");
  await expect(shell).not.toContainText("Bearer ");
  await expect(shell).not.toContainText("ticket=");
});
