import { defineConfig, devices } from "@playwright/test";

const testServer = "http://127.0.0.1:4331";

export default defineConfig({
  testDir: "./tests/e2e",
  fullyParallel: true,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  reporter: process.env.CI ? [["github"], ["html", { open: "never" }]] : "list",
  use: {
    baseURL: `${testServer}/kongctl/`,
    trace: "on-first-retry",
  },
  webServer: {
    command: "npm run preview -- --host 127.0.0.1 --port 4331",
    url: `${testServer}/kongctl/`,
    reuseExistingServer: !process.env.CI,
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
