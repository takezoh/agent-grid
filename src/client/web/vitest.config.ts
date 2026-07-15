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
        lines: 88,
        statements: 85,
        functions: 88,
        branches: 75,
        "src/{api,socket,wire}/**": {
          lines: 86,
          statements: 81,
          functions: 80,
          branches: 70,
        },
        "src/{store,hooks}/**": {
          lines: 92,
          statements: 90,
          functions: 93,
          branches: 83,
        },
        "src/components/**": {
          lines: 86,
          statements: 83,
          functions: 88,
          branches: 73,
        },
        "src/{lib,util}/**": {
          lines: 87,
          statements: 85,
          functions: 96,
          branches: 76,
        },
      },
    },
  },
});
