import { defineConfig } from "@playwright/test";

// Expects a running server (see Makefile target `e2e`), default http://localhost:8080.
export default defineConfig({
  testDir: "./e2e",
  timeout: 30_000,
  use: {
    baseURL: process.env.KYBER_E2E_URL ?? "http://localhost:8080",
    launchOptions: process.env.KYBER_CHROMIUM ? { executablePath: process.env.KYBER_CHROMIUM } : {},
  },
});
