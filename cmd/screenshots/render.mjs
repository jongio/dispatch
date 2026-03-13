// render.mjs — Convert HTML screenshot files to PNGs using Playwright.
//
// Usage: node cmd/screenshots/render.mjs [dir]
//
// Reads .html files from [dir] (default: web/public/screenshots),
// including subdirectories, opens each in a headless Chromium browser,
// takes a screenshot of the <pre> element, and saves it as a .png
// alongside the .html. The .html files are deleted after conversion.

import { chromium } from 'playwright';
import { readdir, unlink } from 'node:fs/promises';
import { join, relative, resolve } from 'node:path';

const dir = resolve(process.argv[2] || 'web/public/screenshots');

// Recursively find all .html files under dir.
async function findHTML(root) {
  const entries = await readdir(root, { withFileTypes: true });
  const results = [];
  for (const e of entries) {
    const full = join(root, e.name);
    if (e.isDirectory()) {
      results.push(...await findHTML(full));
    } else if (e.name.endsWith('.html')) {
      results.push(full);
    }
  }
  return results;
}

const htmlFiles = await findHTML(dir);
if (htmlFiles.length === 0) {
  console.error(`No .html files found in ${dir}`);
  process.exit(1);
}

console.log(`Rendering ${htmlFiles.length} screenshots from ${dir}`);

const browser = await chromium.launch();
const context = await browser.newContext({
  deviceScaleFactor: 2,       // 2× for Retina-quality PNGs
  viewport: { width: 1400, height: 900 },
});

let failed = 0;
for (let i = 0; i < htmlFiles.length; i++) {
  const htmlPath = htmlFiles[i];
  const pngPath = htmlPath.replace(/\.html$/, '.png');
  const label = relative(dir, pngPath);

  try {
    const page = await context.newPage();
    await page.goto(`file://${htmlPath}`, { waitUntil: 'load' });

    // Screenshot the .terminal container (handles both normal and overlay).
    const terminal = await page.locator('.terminal').first();
    await terminal.screenshot({ path: pngPath, type: 'png' });
    await page.close();

    // Clean up the intermediate HTML file.
    await unlink(htmlPath);

    console.log(`  [${i + 1}/${htmlFiles.length}] ${label}`);
  } catch (err) {
    console.error(`  [${i + 1}/${htmlFiles.length}] FAIL ${label}: ${err.message}`);
    failed++;
  }
}

await browser.close();

if (failed > 0) {
  console.error(`\n${failed} of ${htmlFiles.length} screenshots failed`);
  process.exit(1);
}

console.log(`\nAll ${htmlFiles.length} screenshots saved to ${dir}`);
