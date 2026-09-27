import { defineConfig, devices } from "@playwright/test";
export default defineConfig({
  testDir: "./e2e",
  workers: 1,
  timeout: 45000,
  use: {
    baseURL: "http://127.0.0.1:18082/fabricworld/",
    headless: true,
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "webkit-controls",
      testMatch: "**/controls.spec.ts",
      use: { ...devices["iPhone 13"], browserName: "webkit" },
    },
    {
      name: "chromium",
      use: {
        browserName: "chromium",
        channel: process.env.PW_CHANNEL || "chrome",
      },
    },
    {
      name: "webkit-works",
      testMatch: "**/works.spec.ts",
      use: { ...devices["iPhone 13"], browserName: "webkit" },
    },
    {
      name: "webkit-purchase",
      testMatch: "**/purchase.spec.ts",
      use: { ...devices["iPhone 13"], browserName: "webkit" },
    },
    {
      name: "webkit-materials",
      testMatch: ["**/materials.spec.ts", "**/material-menu.spec.ts"],
      use: { ...devices["iPhone 13"], browserName: "webkit" },
    },
    {
      name: "webkit-photos",
      testMatch: ["**/photo-preview.spec.ts", "**/photo-viewer.spec.ts"],
      use: { ...devices["iPhone 13"], browserName: "webkit" },
    },
  ],
  reporter: "list",
});
