import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  timeout: 45000,
  use: {
    baseURL: "http://127.0.0.1:18082/fabricworld/",
    channel: process.env.PW_CHANNEL || "chrome",
    headless: true,
    screenshot: "only-on-failure",
  },
  reporter: "list",
});
