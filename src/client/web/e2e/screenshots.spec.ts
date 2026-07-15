/**
 * screenshots.spec.ts — opt-in visual survey for web-ui-refresh (NFR-003).
 *
 * Default CI / `npm run test:e2e` skips these tests. Run manually:
 *
 *   AG_E2E_SCREENSHOTS=1 npx playwright test e2e/screenshots.spec.ts
 *
 * Update baselines after intentional UI changes:
 *
 *   AG_E2E_SCREENSHOTS=1 npx playwright test e2e/screenshots.spec.ts --update-snapshots
 */

import { applyTheme, bootstrapScreenshotApp } from "./support/screenshot-fixture";
import { expect, test } from "./support/test";

const OPT_IN = process.env.AG_E2E_SCREENSHOTS === "1";

test.describe("web-ui-refresh screenshot survey (opt-in)", () => {
  test.skip(!OPT_IN, "Set AG_E2E_SCREENSHOTS=1 to run the visual survey");

  test.beforeEach(async ({ page }) => {
    await page.emulateMedia({ reducedMotion: "reduce" });
    await bootstrapScreenshotApp(page);
    await expect(
      page.locator(".header-bar__title", { hasText: "Fix WS reconnect backoff" }),
    ).toBeVisible();
  });

  test("shell — dark desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await applyTheme(page, "dark");
    await expect(page.locator(".app-shell")).toHaveScreenshot("shell-dark-desktop.png", {
      animations: "disabled",
    });
  });

  test("shell — light desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await applyTheme(page, "light");
    await expect(page.locator(".app-shell")).toHaveScreenshot("shell-light-desktop.png", {
      animations: "disabled",
    });
  });

  test("shell — dark mobile", async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await applyTheme(page, "dark");
    await expect(page.locator(".app-shell")).toHaveScreenshot("shell-dark-mobile.png", {
      animations: "disabled",
    });
  });

  test("palette — dark desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await applyTheme(page, "dark");
    await page.getByRole("button", { name: "Open command menu" }).click();
    await expect(page.getByRole("dialog", { name: "Command Palette" })).toBeVisible();
    await expect(page.getByTestId("palette-overlay")).toHaveScreenshot("palette-dark-desktop.png", {
      animations: "disabled",
    });
  });

  test("workspace mode — light desktop", async ({ page }) => {
    await page.setViewportSize({ width: 1280, height: 800 });
    await applyTheme(page, "light");
    await page.getByRole("radio", { name: "Workspace" }).click();
    await expect(page.getByTestId("workspace-drawer")).toBeVisible();
    await expect(page.locator(".app-shell")).toHaveScreenshot("workspace-light-desktop.png", {
      animations: "disabled",
    });
  });
});
