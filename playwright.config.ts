import { defineConfig } from "@playwright/test";

export default defineConfig({
  testDir: "./tests/e2e",
  outputDir: "./.tmp/playwright-test-results",
  fullyParallel: false,
  workers: 1,
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? "line" : "list",
  use: {
    baseURL: "http://127.0.0.1:8096",
    trace: "retain-on-failure",
    screenshot: "only-on-failure",
  },
  webServer: {
    command: "VITE_E2E=1 pnpm --filter console run build && bash scripts/e2e/run-server.sh",
    url: "http://127.0.0.1:8096/console/",
    reuseExistingServer: false,
    // CI 的全新 runner 需要编译 Go 服务；将冷启动时间与测试超时解耦。
    timeout: process.env.CI ? 300_000 : 120_000,
  },
});
