import { expect, test, type Page } from '@playwright/test';

const pages = [
  { path: './', heading: 'dispatch' },
  { path: 'features/', heading: 'Features' },
  { path: 'install/', heading: 'Install' },
  { path: 'cli/', heading: 'CLI Reference' },
  { path: 'config/', heading: 'Configuration' },
  { path: 'keys/', heading: 'Keyboard & Mouse' },
  { path: 'changelog/', heading: 'Changelog' },
];

async function expectCleanPage(page: Page, path: string, heading: string) {
  const consoleErrors: string[] = [];
  const pageErrors: string[] = [];
  const failedResponses: string[] = [];

  page.on('console', message => {
    if (message.type() === 'error') {
      consoleErrors.push(message.text());
    }
  });
  page.on('pageerror', error => pageErrors.push(error.message));
  page.on('response', response => {
    if (response.status() >= 400) {
      failedResponses.push(`${response.status()} ${response.url()}`);
    }
  });

  const response = await page.goto(path);
  expect(response?.status()).toBe(200);
  await expect(page.getByRole('heading', { level: 1, name: heading })).toBeVisible();
  await expect(page.locator('main')).not.toBeEmpty();
  expect(consoleErrors).toEqual([]);
  expect(pageErrors).toEqual([]);
  expect(failedResponses).toEqual([]);
}

for (const pageInfo of pages) {
  test(`${pageInfo.heading} page renders without runtime failures`, async ({ page }) => {
    await expectCleanPage(page, pageInfo.path, pageInfo.heading);
  });
}

test('primary navigation reaches the features page', async ({ page, isMobile }) => {
  await page.goto('./');

  if (isMobile) {
    const menuToggle = page.getByRole('button', { name: 'Toggle menu' });
    await menuToggle.click();
    await expect(menuToggle).toHaveAttribute('aria-expanded', 'true');
  }
  await page.getByRole('link', { name: 'Features', exact: true }).first().click();

  await expect(page).toHaveURL(/\/dispatch\/features\/$/);
  await expect(page.getByRole('heading', { level: 1, name: 'Features' })).toBeVisible();
});

test('skip link moves focus to main content', async ({ page, browserName }) => {
  await page.goto('./');

  const skipLink = page.getByRole('link', { name: 'Skip to main content' });
  if (browserName === 'webkit' && process.platform === 'win32') {
    // WebKit on Windows does not enable link tabbing without the host OS
    // full-keyboard-access setting, so start from the same focused control.
    await skipLink.focus();
  } else {
    await page.keyboard.press('Tab');
  }
  await expect(skipLink).toBeFocused();
  await skipLink.press('Enter');

  await expect(page.locator('main')).toBeFocused();
});
