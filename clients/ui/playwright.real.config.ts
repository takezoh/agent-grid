import { defineConfig, devices } from "@playwright/test";

const webAddress = process.env.AG_UI_E2E_WEB_ADDR ?? "127.0.0.1:18081";
const baseURL = `http://${webAddress}`;

export default defineConfig({
  testDir: "./e2e/real-stack",
  fullyParallel: false,
  retries: 0,
  reporter: process.env.CI ? "github" : "list",
  timeout: 60_000,
  use: {
    baseURL,
    trace: "retain-on-failure",
  },
  webServer: {
    command: "./scripts/e2e-real-stack.sh",
    url: baseURL,
    reuseExistingServer: false,
    timeout: 120_000,
    stdout: "pipe",
    stderr: "pipe",
  },
  projects: [
    {
      name: "chromium-real-stack",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
