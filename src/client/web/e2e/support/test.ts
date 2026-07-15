import { test as base, expect } from "@playwright/test";

type BrowserDiagnostic = {
  kind: "console" | "pageerror";
  message: string;
};

/**
 * Shared browser-fidelity guard. Browser warnings, errors, and uncaught page
 * errors are test failures instead of retry-masked diagnostics.
 */
export const test = base.extend({
  page: async ({ page }, use) => {
    const diagnostics: BrowserDiagnostic[] = [];
    page.on("console", (message) => {
      if (message.type() === "warning" || message.type() === "error") {
        diagnostics.push({ kind: "console", message: message.text() });
      }
    });
    page.on("pageerror", (error) => {
      diagnostics.push({ kind: "pageerror", message: error.message });
    });

    await use(page);

    expect(diagnostics, "unexpected browser console/page error output").toEqual([]);
  },
});

export { expect };
