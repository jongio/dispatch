import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// These tests verify the shell detection functions work correctly
// by mocking filesystem and process lookups.

// Mock fs.existsSync with case-insensitive matching for Windows paths
vi.mock('fs', async (importOriginal) => {
  const actual = await importOriginal<typeof import('fs')>();
  return {
    ...actual,
    existsSync: vi.fn((path: unknown) => {
      if (typeof path !== 'string') return false;
      const normalized = path.toLowerCase().replace(/\//g, '\\');
      const existing = [
        'c:\\program files\\powershell\\7\\pwsh.exe',
        'c:\\windows\\system32\\windowspowershell\\v1.0\\powershell.exe',
        'c:\\windows\\system32\\cmd.exe',
        'c:\\program files\\git\\bin\\bash.exe',
        'c:\\windows\\system32\\wsl.exe',
        // For terminal detection (wt.exe found via findOnPath)
      ];
      return existing.some((p) => normalized === p || normalized.endsWith(p.split('\\').pop()!));
    }),
  };
});

vi.mock('child_process', () => ({
  execFileSync: vi.fn((cmd: string, args: string[]) => {
    const binary = args[0];
    const found: Record<string, string> = {
      'wt.exe': 'C:\\Users\\test\\AppData\\Local\\Microsoft\\WindowsApps\\wt.exe\n',
      'pwsh.exe': 'C:\\Program Files\\PowerShell\\7\\pwsh.exe\n',
    };
    if (found[binary]) return found[binary];
    throw new Error(`INFO: Could not find files for the given pattern(s).`);
  }),
}));

describe('shells module', () => {
  let originalPlatform: PropertyDescriptor | undefined;

  beforeEach(() => {
    originalPlatform = Object.getOwnPropertyDescriptor(process, 'platform');
    vi.resetModules();
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

      const { getShells } = await import('../src/main/shells');
      const shells = getShells();

      expect(shells.length).toBeGreaterThanOrEqual(2);

      const shellNames = shells.map((s) => s.name);
      // At minimum, pwsh and cmd should be detected via our mocks
      expect(shellNames).toContain('pwsh');
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

      if (shells.length > 0) {
        expect(shells[0].isDefault).toBe(true);
      }
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
    it('detects terminals with correct capability flags', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      // Also mock existsSync for wt.exe path returned by findOnPath
      const fsMod = vi.mocked(await import('fs'));
      const origExistsSync = fsMod.existsSync;
      fsMod.existsSync = vi.fn((path: unknown) => {
        if (typeof path === 'string' && path.includes('WindowsApps')) return true;
        return origExistsSync(path);
      }) as any;

      const { getTerminals } = await import('../src/main/shells');
      const terminals = getTerminals();

      expect(terminals.length).toBeGreaterThan(0);

      // All terminals should have the required interface
      for (const terminal of terminals) {
        expect(terminal.name).toBeTruthy();
        expect(terminal.path).toBeTruthy();
        expect(terminal.displayName).toBeTruthy();
        expect(typeof terminal.isDefault).toBe('boolean');
        expect(typeof terminal.supportsNewTab).toBe('boolean');
        expect(typeof terminal.supportsSplitPane).toBe('boolean');
      }
    });

    it('Windows Terminal supports new tabs and split panes', async () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      const fsMod = vi.mocked(await import('fs'));
      fsMod.existsSync = vi.fn((path: unknown) => {
        if (typeof path === 'string' && path.includes('WindowsApps')) return true;
        if (typeof path === 'string' && path.toLowerCase().includes('cmd.exe')) return true;
        return false;
      }) as any;

      const { getTerminals } = await import('../src/main/shells');
      const terminals = getTerminals();
      const wt = terminals.find((t) => t.name === 'windows-terminal');

      expect(wt).toBeDefined();
      expect(wt!.supportsNewTab).toBe(true);
      expect(wt!.supportsSplitPane).toBe(true);
    });
  });
});