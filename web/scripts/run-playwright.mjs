import { spawn } from 'node:child_process';
import { createServer } from 'node:net';

const host = '127.0.0.1';
const port = await findAvailablePort();
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
    '--strictPort',
  ],
  { stdio: 'inherit', windowsHide: true },
);

async function findAvailablePort() {
  const listener = createServer();
  await new Promise((resolve, reject) => {
    listener.once('error', reject);
    listener.listen(0, host, resolve);
  });

  const address = listener.address();
  if (!address || typeof address === 'string') {
    listener.close();
    throw new Error('Could not allocate a local port for Astro preview');
  }

  await new Promise((resolve, reject) => {
    listener.close(error => {
      if (error) {
        reject(error);
      } else {
        resolve();
      }
    });
  });
  return String(address.port);
}

async function waitForServer() {
  const deadline = Date.now() + 120_000;
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
      env: {
        ...process.env,
        DISPATCH_EXTERNAL_WEB_SERVER: '1',
        DISPATCH_BASE_URL: baseURL,
      },
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
