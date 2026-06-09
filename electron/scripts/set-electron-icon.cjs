#!/usr/bin/env node
/**
 * set-electron-icon — embed build/icon.ico into the dev electron.exe so the
 * Windows taskbar/alt-tab uses Dispatch's icon instead of the default Electron
 * one. Production builds get the icon via electron-builder; this script
 * fixes the dev-mode experience.
 *
 * No-op on non-Windows platforms.
 */
const path = require('node:path');
const fs = require('node:fs');

async function main() {
  if (process.platform !== 'win32') {
    console.log('[icon:dev] skipped — only needed on Windows');
    return;
  }

  const electronPkg = path.dirname(require.resolve('electron/package.json'));
  const pathFile = path.join(electronPkg, 'path.txt');
  if (!fs.existsSync(pathFile)) {
    console.warn(
      `[icon:dev] electron path.txt missing at ${pathFile} — ` +
        'binary may not be downloaded yet. Run `pnpm run icon:dev` later.',
    );
    return;
  }
  const exeRel = fs.readFileSync(pathFile, 'utf8').trim();
  const exePath = path.join(electronPkg, 'dist', exeRel);
  if (!fs.existsSync(exePath)) {
    console.error(`[icon:dev] electron.exe not found at ${exePath}`);
    process.exit(1);
  }

  const iconPath = path.resolve(__dirname, '..', 'build', 'icon.ico');
  if (!fs.existsSync(iconPath)) {
    console.error(`[icon:dev] icon missing at ${iconPath}`);
    process.exit(1);
  }

  const { rcedit } = require('rcedit');
  console.log(`[icon:dev] writing ${iconPath} → ${exePath}`);
  await rcedit(exePath, {
    icon: iconPath,
    'version-string': {
      ProductName: 'Dispatch',
      FileDescription: 'Dispatch',
      CompanyName: 'Jon Gallant',
    },
  });
  console.log('[icon:dev] done');
}

main().catch((err) => {
  const stderr = String(err?.stderr ?? '');
  if (stderr.includes('Unable to commit changes')) {
    console.warn(
      '[icon:dev] electron.exe is locked (is dispatch running?). Skipping. ' +
        'Close dispatch and run `pnpm run icon:dev` to retry.',
    );
    process.exit(0);
  }
  console.error('[icon:dev] failed:', err);
  process.exit(1);
});
