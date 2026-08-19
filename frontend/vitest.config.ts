import { defineConfig } from "vitest/config";
import solid from "vite-plugin-solid";
import path from "node:path";

// Vitest gets its own config so the Solid plugin can be configured for test
// transforms without touching the production build pipeline.
export default defineConfig({
  plugins: [solid()],
  resolve: {
    alias: {
      "~": path.resolve(__dirname, "src"),
    },
    conditions: ["development", "browser"],
  },
  test: {
    environment: "jsdom",
    include: ["src/**/*.test.{ts,tsx}"],
    globals: false,
    coverage: {
      provider: "v8",
      include: ["src/**/*.{ts,tsx}"],
      exclude: ["src/**/*.test.{ts,tsx}", "src/components/ui/**"],
    },
  },
});
