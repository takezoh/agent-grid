import { expect, test } from "@playwright/test";
import { installFakeBackend, makeSessionInfo } from "./support/fake-backend";

// Browser wiring that happy-dom cannot prove: a native contextmenu event on a
// session row opens the Radix context menu, and the Stop item hands off to
// the App-owned ConfirmDialog (focus returns to the row on cancel).
test("right-click on a session row opens the session context menu", async ({ page }) => {
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

  const row = page.locator('.session-list__row[data-session-id="session-1"]');
  await expect(row).toBeVisible();
  await row.click({ button: "right" });

  const menu = page.locator(".session-context-menu");
  await expect(menu).toBeVisible();
  await expect(menu.getByText("Open")).toBeVisible();
  await expect(menu.getByText("Copy session ID")).toBeVisible();

  // Stop session… routes into the App-owned ConfirmDialog.
  await menu.getByText("Stop session…").click();
  const dialog = page.getByRole("dialog").filter({ hasText: "Stop session" });
  await expect(dialog).toBeVisible();
  await expect(
    dialog.getByText('"Existing session" will be stopped', { exact: false }),
  ).toBeVisible();

  // Cancel closes the dialog without touching the session.
  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).not.toBeVisible();
});

// A session title without an upstream length cap must not blow out the
// context menu or the confirm dialog layout; both truncate to a fixed
// character count with an ellipsis instead of growing unbounded.
test("a very long session title is truncated in the context menu and the stop confirm dialog", async ({
  page,
}) => {
  const longTitle = "Extremely long session title ".repeat(10).trim();
  const backend = await installFakeBackend(page, {
    sessions: [
      makeSessionInfo({
        id: "session-1",
        project: "/repo/app",
        command: "claude",
        title: longTitle,
        status: "running",
      }),
    ],
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
        title: longTitle,
        status: "running",
      }),
    ],
    activeSessionID: "session-1",
    features: ["surface"],
    serverTime: 1_720_000_000_000,
  });

  const row = page.locator('.session-list__row[data-session-id="session-1"]');
  await expect(row).toBeVisible();
  await row.click({ button: "right" });

  const menu = page.locator(".session-context-menu");
  const menuLabel = menu.locator(".overflow-menu__label");
  await expect(menuLabel).toBeVisible();
  await expect(menuLabel).not.toHaveText(longTitle);
  // Full title is still reachable via the title attribute (hover tooltip).
  await expect(menuLabel).toHaveAttribute("title", longTitle);

  await menu.getByText("Stop session…").click();
  const dialog = page.getByRole("dialog").filter({ hasText: "Stop session" });
  await expect(dialog).toBeVisible();
  await expect(dialog.getByText(longTitle, { exact: false })).toHaveCount(0);
  await expect(dialog.getByText("will be stopped", { exact: false })).toBeVisible();

  await dialog.getByRole("button", { name: "Cancel" }).click();
  await expect(dialog).not.toBeVisible();
});
