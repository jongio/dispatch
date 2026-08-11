import { defineConfig, devices } from '@playwright/test';

export default defineConfig({
  testDir: './tests',
  fullyParallel: false,
  forbidOnly: Boolean(process.env.CI),
  retries: process.env.CI ? 2 : 0,
  workers: process.env.CI ? 2 : 4,
  reporter: 'line',
  use: {
    baseURL: 'http://127.0.0.1:4321/dispatch/',
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
        url: 'http://127.0.0.1:4321/dispatch/',
        reuseExistingServer: false,
      },
});
