import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// These tests verify the shell detection functions work correctly
// by mocking filesystem and process lookups.

// Mock fs.existsSync and child_process.execFileSync
vi.mock('fs', async (importOriginal) => {
  const actual = await importOriginal<typeof import('fs')>();
  return {
    ...actual,
    existsSync: vi.fn((path: string) => {
      // Simulate Windows environment
      const existing = new Set([
        'C:\\Program Files\\PowerShell\\7\\pwsh.exe',
        'C:\\Windows\\System32\\WindowsPowerShell\\v1.0\\powershell.exe',
        'C:\\Windows\\System32\\cmd.exe',
        'C:\\Program Files\\Git\\bin\\bash.exe',
        'C:\\Windows\\System32\\wsl.exe',
      ]);
      return existing.has(path);
    }),
  };
});

vi.mock('child_process', () => ({
  execFileSync: vi.fn((cmd: string, args: string[]) => {
    // Simulate `where.exe` for Windows
    const found: Record<string, string> = {
      'wt.exe': 'C:\\Users\\test\\AppData\\Local\\Microsoft\\WindowsApps\\wt.exe\n',
      'pwsh.exe': 'C:\\Program Files\\PowerShell\\7\\pwsh.exe\n',
    };
    const binary = args[0];
    if (found[binary]) return found[binary];
    throw new Error(`not found: ${binary}`);
  }),
}));

describe('shells module', () => {
  let originalPlatform: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalPlatform = Object.getOwnPropertyDescriptor(process, 'platform');
  });

  afterEach(() => {
    if (originalPlatform) {
      Object.defineProperty(process, 'platform', originalPlatform);
    }
    vi.clearAllMocks();
  });

  describe('getShells (Windows)', () => {
    it('detects available Windows shells', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      // Re-import to get fresh module with mocked fs
      const { getShells } = await import('../src/main/shells');
      const shells = getShells();

      expect(shells.length).toBeGreaterThan(0);

      const shellNames = shells.map((s) => s.name);
      expect(shellNames).toContain('pwsh');
      expect(shellNames).toContain('powershell');
      expect(shellNames).toContain('cmd');
      expect(shellNames).toContain('git-bash');
      expect(shellNames).toContain('wsl');
    });

    it('marks exactly one shell as default', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      const { getShells } = await import('../src/main/shells');
      const shells = getShells();
      const defaults = shells.filter((s) => s.isDefault);

      expect(defaults).toHaveLength(1);
    });

    it('puts default shell first in the list', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      const { getShells } = await import('../src/main/shells');
      const shells = getShells();

      expect(shells[0].isDefault).toBe(true);
    });

    it('each shell has required fields', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      const { getShells } = await import('../src/main/shells');
      const shells = getShells();

      for (const shell of shells) {
        expect(shell.name).toBeTruthy();
        expect(shell.path).toBeTruthy();
        expect(shell.displayName).toBeTruthy();
        expect(typeof shell.isDefault).toBe('boolean');
      }
    });
  });

  describe('getTerminals (Windows)', () => {
    it('detects Windows Terminal when available', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      // Need to mock existsSync for the wt.exe path returned by findOnPath
      const fs = vi.mocked(await import('fs'));
      const originalExistsSync = fs.existsSync;
      fs.existsSync = vi.fn((path: string) => {
        if (typeof path === 'string' && path.includes('WindowsApps')) return true;
        return (originalExistsSync as (p: string) => boolean)(path);
      }) as unknown as typeof fs.existsSync;

      const { getTerminals } = await import('../src/main/shells');
      const terminals = getTerminals();

      expect(terminals.length).toBeGreaterThan(0);
      const wt = terminals.find((t) => t.name === 'windows-terminal');
      expect(wt).toBeDefined();
      expect(wt!.supportsNewTab).toBe(true);
      expect(wt!.supportsSplitPane).toBe(true);
    });

    it('each terminal has required fields', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      const { getTerminals } = await import('../src/main/shells');
      const terminals = getTerminals();

      for (const terminal of terminals) {
        expect(terminal.name).toBeTruthy();
        expect(terminal.path).toBeTruthy();
        expect(terminal.displayName).toBeTruthy();
        expect(typeof terminal.isDefault).toBe('boolean');
        expect(typeof terminal.supportsNewTab).toBe('boolean');
        expect(typeof terminal.supportsSplitPane).toBe('boolean');
      }
    });
  });
});
