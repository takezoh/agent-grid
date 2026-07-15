import react from "@vitejs/plugin-react-swc";
import { defineConfig } from "vitest/config";

export default defineConfig({
  plugins: [react()],
  test: {
    environment: "happy-dom",
    globals: true,
    setupFiles: ["./src/test-setup.ts"],
    exclude: ["e2e/**", "node_modules/**"],
    coverage: {
      provider: "v8",
      reporter: ["text", "json-summary"],
      reportsDirectory: "coverage",
      include: ["src/**/*.{ts,tsx}"],
      exclude: [
        "src/**/*.test.{ts,tsx}",
        "src/**/__tests__/**",
        "src/main.tsx",
        "src/vite-env.d.ts",
      ],
      thresholds: {
        lines: 90,
        statements: 90,
        functions: 87,
        branches: 85,
        "src/{api,socket,wire}/**": {
          lines: 87,
          statements: 87,
          functions: 86,
          branches: 80,
        },
        "src/{store,hooks}/**": {
          lines: 93,
          statements: 93,
          functions: 92,
          branches: 88,
        },
        "src/components/**": {
          lines: 90,
          statements: 90,
          functions: 86,
          branches: 85,
        },
        "src/{lib,util}/**": {
          lines: 87,
          statements: 87,
          functions: 94,
          branches: 83,
        },
      },
    },
  },
});
