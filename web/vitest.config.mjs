import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],

  test: {
    environment: "jsdom",
    setupFiles: ["./test/setup.js"],
    include: ["test/**/*.test.{js,jsx}"],

    coverage: {
      provider: "v8",
      reporter: ["text", "lcov"],
      reportsDirectory: "./coverage",

      // Behavioral production code must stay measurable.
      include: [
        "lib/**/*.js",
        "components/LiveChatProvider.jsx",
        "components/SessionGate.jsx",
        "app/login/page.jsx",
        "app/page.jsx",
        "app/workforce/page.jsxx",
        "app/inventory/page.jsx",
      ],

      // Legitimate exclusions only:
      // - test/build artifacts
      // - pure presentation/decorative components with no business behavior.
      exclude: [
        "test/**",
        "coverage/**",
        "node_modules/**",
        ".next/**",
        "components/TropicalAtmosphere.jsx",
        "components/EnterpriseLoader.jsx",
      ],

      thresholds: {
        lines: 80,
        functions: 80,
        statements: 80,
        branches: 75,
      },
    },
  },
});
