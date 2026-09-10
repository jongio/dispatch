import { defineConfig, devices } from '@playwright/test';

const baseURL = process.env.DISPATCH_BASE_URL ?? 'http://127.0.0.1:4321/dispatch/';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : 4,
  reporter: 'line',
  use: {
    baseURL,
    trace: 'on-first-retry',
  },
  projects: [
    {
      name: 'chromium',
      use: { ...devices['Desktop Chrome'] },
    },
    {
      name: 'firefox',
      use: { ...devices['Desktop Firefox'] },
    },
    {
      name: 'webkit',
      use: { ...devices['Desktop Safari'] },
    },
    {
      name: 'mobile-chromium',
      use: { ...devices['Pixel 7'] },
    },
  ],
  webServer: process.env.DISPATCH_EXTERNAL_WEB_SERVER
    ? undefined
    : {
        command:
          'node ./node_modules/astro/bin/astro.mjs preview --host 127.0.0.1 --port 4321',
        url: baseURL,
        reuseExistingServer: false,
      },
});
