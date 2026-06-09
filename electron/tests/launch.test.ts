import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { LaunchManager } from '../src/main/launch';
import type { LaunchOptions } from '../src/main/launch';

// Mock child_process
vi.mock('child_process', () => ({
  spawn: vi.fn(() => ({ unref: vi.fn() })),
  execFile: vi.fn(),
  execFileSync: vi.fn(() => 'C:\\Program Files\\PowerShell\\7\\pwsh.exe\n'),
}));

// Mock shells module to return predictable results
vi.mock('../src/main/shells', () => ({
  getShells: vi.fn(() => [
    { name: 'pwsh', path: 'C:\\Program Files\\PowerShell\\7\\pwsh.exe', displayName: 'PowerShell 7', isDefault: true },
    { name: 'cmd', path: 'C:\\Windows\\System32\\cmd.exe', displayName: 'Command Prompt', isDefault: false },
  ]),
  getTerminals: vi.fn(() => [
    { name: 'windows-terminal', path: 'C:\\Users\\test\\AppData\\Local\\Microsoft\\WindowsApps\\wt.exe', displayName: 'Windows Terminal', isDefault: true, supportsNewTab: true, supportsSplitPane: true },
    { name: 'cmd', path: 'C:\\Windows\\System32\\cmd.exe', displayName: 'Command Prompt Window', isDefault: false, supportsNewTab: false, supportsSplitPane: false },
  ]),
}));

describe('LaunchManager', () => {
  let launcher: LaunchManager;
  let spawnMock: ReturnType<typeof vi.fn>;

  beforeEach(() => {
    launcher = new LaunchManager();
    const cp = vi.mocked(await import('child_process'));
    spawnMock = cp.spawn as ReturnType<typeof vi.fn>;
    spawnMock.mockClear();
    spawnMock.mockReturnValue({ unref: vi.fn() });
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('buildResumeCommand (via launchInPlace)', () => {
    it('builds basic resume command', () => {
      launcher.launchInPlace('abc-123');

      expect(spawnMock).toHaveBeenCalledWith(
        'C:\\Program Files\\PowerShell\\7\\pwsh.exe',
        ['-NoProfile', '-Command', 'gh copilot session resume abc-123'],
        expect.objectContaining({ detached: true }),
      );
    });

    it('includes --yolo flag when yoloMode is true', () => {
      launcher.launchInPlace('abc-123', { yoloMode: true });

      expect(spawnMock).toHaveBeenCalledWith(
        expect.any(String),
        expect.arrayContaining([expect.stringContaining('--yolo')]),
        expect.anything(),
      );
    });

    it('includes --agent flag when specified', () => {
      launcher.launchInPlace('abc-123', { agent: 'developer' });

      const args = spawnMock.mock.calls[0][1];
      const command = args[args.length - 1];
      expect(command).toContain('--agent developer');
    });

    it('includes --model flag when specified', () => {
      launcher.launchInPlace('abc-123', { model: 'claude-sonnet-4' });

      const args = spawnMock.mock.calls[0][1];
      const command = args[args.length - 1];
      expect(command).toContain('--model claude-sonnet-4');
    });

    it('uses customCommand when provided', () => {
      launcher.launchInPlace('abc-123', { customCommand: 'echo hello' });

      const args = spawnMock.mock.calls[0][1];
      const command = args[args.length - 1];
      expect(command).toBe('echo hello');
    });

    it('combines all flags correctly', () => {
      launcher.launchInPlace('sess-id', {
        yoloMode: true,
        agent: 'tester',
        model: 'gpt-5',
      });

      const args = spawnMock.mock.calls[0][1];
      const command = args[args.length - 1];
      expect(command).toBe('gh copilot session resume sess-id --yolo --agent tester --model gpt-5');
    });
  });

  describe('shell resolution', () => {
    it('uses default shell when no shell specified', () => {
      launcher.launchInPlace('test-id');

      // pwsh is the default
      expect(spawnMock.mock.calls[0][0]).toBe('C:\\Program Files\\PowerShell\\7\\pwsh.exe');
    });

    it('uses specified shell by name', () => {
      launcher.launchInPlace('test-id', { shell: 'cmd' });

      expect(spawnMock.mock.calls[0][0]).toBe('C:\\Windows\\System32\\cmd.exe');
    });

    it('returns error when shell not found', () => {
      const result = launcher.launchInPlace('test-id', { shell: 'nonexistent' });

      expect(result.success).toBe(false);
      expect(result.error).toContain('No suitable shell');
    });
  });

  describe('shell argument building', () => {
    it('uses -NoProfile -Command for pwsh', () => {
      launcher.launchInPlace('id', { shell: 'pwsh' });

      const args = spawnMock.mock.calls[0][1];
      expect(args[0]).toBe('-NoProfile');
      expect(args[1]).toBe('-Command');
    });

    it('uses /c for cmd', () => {
      launcher.launchInPlace('id', { shell: 'cmd' });

      const args = spawnMock.mock.calls[0][1];
      expect(args[0]).toBe('/c');
    });
  });

  describe('launchNewTab', () => {
    it('uses Windows Terminal wt.exe with new-tab args', () => {
      // Mock process.platform
      Object.defineProperty(process, 'platform', { value: 'win32' });

      launcher.launchNewTab('test-id');

      const [binary, args] = [spawnMock.mock.calls[0][0], spawnMock.mock.calls[0][1]];
      expect(binary).toContain('wt.exe');
      expect(args).toContain('-w');
      expect(args).toContain('0');
      expect(args).toContain('new-tab');
    });
  });

  describe('launchSplitPane', () => {
    it('returns error when terminal does not support split panes', () => {
      const result = launcher.launchSplitPane('test-id', { terminal: 'cmd' });

      expect(result.success).toBe(false);
      expect(result.error).toContain('not supported');
    });

    it('uses split-pane with -H for horizontal direction', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });

      launcher.launchSplitPane('test-id', { paneDirection: 'horizontal' });

      const args = spawnMock.mock.calls[0][1];
      expect(args).toContain('split-pane');
      expect(args).toContain('-H');
    });

    it('uses split-pane with -V for vertical direction', () => {
      Object.defineProperty(process, 'platform', { value: 'win32' });

      launcher.launchSplitPane('test-id', { paneDirection: 'vertical' });

      const args = spawnMock.mock.calls[0][1];
      expect(args).toContain('split-pane');
      expect(args).toContain('-V');
    });
  });

  describe('launchMulti', () => {
    it('launches multiple sessions', () => {
      const results = launcher.launchMulti(['id-1', 'id-2', 'id-3'], 'inPlace');

      expect(results).toHaveLength(3);
      expect(results.every((r) => r.success)).toBe(true);
      expect(spawnMock).toHaveBeenCalledTimes(3);
    });

    it('returns individual results for each session', () => {
      // First call succeeds, then we'll test with a nonexistent shell
      const results = launcher.launchMulti(['id-1', 'id-2'], 'inPlace', { shell: 'nonexistent' });

      expect(results).toHaveLength(2);
      expect(results[0].success).toBe(false);
      expect(results[1].success).toBe(false);
    });
  });

  describe('spawn error handling', () => {
    it('handles ENOENT error gracefully', () => {
      spawnMock.mockImplementation(() => {
        const err = new Error('spawn ENOENT') as NodeJS.ErrnoException;
        err.code = 'ENOENT';
        throw err;
      });

      const result = launcher.launchInPlace('test-id');

      expect(result.success).toBe(false);
      expect(result.error).toContain('not found');
    });

    it('handles EACCES error gracefully', () => {
      spawnMock.mockImplementation(() => {
        const err = new Error('spawn EACCES') as NodeJS.ErrnoException;
        err.code = 'EACCES';
        throw err;
      });

      const result = launcher.launchInPlace('test-id');

      expect(result.success).toBe(false);
      expect(result.error).toContain('Permission denied');
    });
  });
});
