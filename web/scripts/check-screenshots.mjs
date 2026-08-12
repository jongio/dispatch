import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const checkDir = mkdtempSync(join(tmpdir(), 'dispatch-screenshots-check-'));

try {
  const result = spawnSync(
    'go',
    ['run', '-tags', 'screenshots', '../cmd/screenshots', '--check', '--out', checkDir],
    { stdio: 'inherit' },
  );

  if (result.error) {
    throw result.error;
  }
  if (result.status !== 0) {
    process.exitCode = result.status ?? 1;
  }
} finally {
  rmSync(checkDir, {
    recursive: true,
    force: true,
    maxRetries: 10,
    retryDelay: 100,
  });
}
