import { spawn } from 'node:child_process';

const host = '127.0.0.1';
const port = '4321';
const baseURL = `http://${host}:${port}/dispatch/`;

const server = spawn(
  process.execPath,
  [
    './node_modules/astro/bin/astro.mjs',
    'preview',
    '--host',
    host,
    '--port',
    port,
  ],
  { stdio: 'inherit', windowsHide: true },
);

async function waitForServer() {
  const deadline = Date.now() + 30_000;
  while (Date.now() < deadline) {
    if (server.exitCode !== null) {
      throw new Error(`Astro preview exited with code ${server.exitCode}`);
    }
    try {
      const response = await fetch(baseURL);
      if (response.ok) {
        return;
      }
    } catch {
      // The server is still starting.
    }
    await new Promise(resolve => setTimeout(resolve, 100));
  }
  throw new Error(`Timed out waiting for ${baseURL}`);
}

async function stopServer() {
  if (server.exitCode !== null) {
    return;
  }
  server.kill();
  await Promise.race([
    new Promise(resolve => server.once('exit', resolve)),
    new Promise(resolve => setTimeout(resolve, 5_000)),
  ]);
}

let exitCode = 1;
try {
  await waitForServer();
  const test = spawn(
    process.execPath,
    ['./node_modules/@playwright/test/cli.js', 'test', ...process.argv.slice(2)],
    {
      stdio: 'inherit',
      windowsHide: true,
      env: { ...process.env, DISPATCH_EXTERNAL_WEB_SERVER: '1' },
    },
  );
  exitCode = await new Promise((resolve, reject) => {
    test.once('error', reject);
    test.once('exit', code => resolve(code ?? 1));
  });
} finally {
  await stopServer();
}

process.exitCode = exitCode;
