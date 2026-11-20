import { defineConfig, devices } from "@playwright/test";

const PORT = Number(process.env.PORT ?? 3100);

export default defineConfig({
  testDir: "./e2e",
  fullyParallel: true,
  reporter: "list",
  // Siembra los usuarios del login si la API local responde (e2e/global-setup.ts).
  globalSetup: "./e2e/global-setup.ts",
  use: {
    baseURL: `http://localhost:${PORT}`,
    trace: "on-first-retry",
  },
  webServer: {
    // E2E_PROD=1 serves the existing production build (`pnpm build` first) instead of the dev server.
    command: process.env.E2E_PROD ? `pnpm next start -p ${PORT}` : `pnpm next dev -p ${PORT}`,
    url: `http://localhost:${PORT}`,
    reuseExistingServer: true,
    timeout: 120_000,
    // El BFF llama a la API local; sin ella los specs del login se saltan.
    env: { WANKA_API_URL: process.env.WANKA_API_URL ?? "http://127.0.0.1:58080" },
  },
  projects: [
    { name: "desktop", use: { ...devices["Desktop Chrome"], viewport: { width: 1440, height: 900 } } },
    { name: "tablet", use: { ...devices["Desktop Chrome"], viewport: { width: 1024, height: 768 } } },
    { name: "mobile", use: { ...devices["Pixel 7"] } },
  ],
});
