import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';
import { existsSync, readFileSync, writeFileSync, mkdirSync, renameSync } from 'fs';
import { load, save, getDefault, getConfigPath } from '../src/main/config';
import type { Config } from '../src/main/config';

vi.mock('fs', () => ({
  existsSync: vi.fn(),
  readFileSync: vi.fn(),
  writeFileSync: vi.fn(),
  mkdirSync: vi.fn(),
  renameSync: vi.fn(),
}));

vi.mock('electron', () => ({
  app: { getPath: vi.fn(() => '/mock') },
  shell: { openPath: vi.fn() },
}));

const existsSyncMock = vi.mocked(existsSync);
const readFileSyncMock = vi.mocked(readFileSync);
const writeFileSyncMock = vi.mocked(writeFileSync);
const mkdirSyncMock = vi.mocked(mkdirSync);
const renameSyncMock = vi.mocked(renameSync);

describe('config', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  describe('getDefault', () => {
    it('returns a complete config with all required fields', () => {
      const config = getDefault();

      expect(config.config_version).toBe(1);
      expect(config.default_shell).toBe('');
      expect(config.default_terminal).toBe('');
      expect(config.default_time_range).toBe('1d');
      expect(config.default_sort).toBe('updated');
      expect(config.default_sort_order).toBe('desc');
      expect(config.default_pivot).toBe('folder');
      expect(config.show_preview).toBe(true);
      expect(config.max_sessions).toBe(100);
      expect(config.yoloMode).toBe(false);
      expect(config.agent).toBe('');
      expect(config.model).toBe('');
      expect(config.custom_command).toBe('');
      expect(config.theme).toBe('');
      expect(config.workspace_recovery).toBe(true);
    });
  });

  describe('load', () => {
    it('returns defaults when config file does not exist', () => {
      existsSyncMock.mockReturnValue(false);

      const config = load();

      expect(config).toEqual(getDefault());
      expect(readFileSyncMock).not.toHaveBeenCalled();
    });

    it('merges partial config with defaults', () => {
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue(JSON.stringify({
        default_shell: 'pwsh',
        yoloMode: true,
      }));

      const config = load();

      expect(config.default_shell).toBe('pwsh');
      expect(config.yoloMode).toBe(true);
      // Unset fields retain defaults
      expect(config.default_time_range).toBe('1d');
      expect(config.max_sessions).toBe(100);
    });

    it('clamps max_sessions above MAX_MAX_SESSIONS to 10000', () => {
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue(JSON.stringify({ max_sessions: 99_999 }));

      const config = load();

      expect(config.max_sessions).toBe(10_000);
    });

    it('clamps negative max_sessions to 0', () => {
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue(JSON.stringify({ max_sessions: -5 }));

      const config = load();

      expect(config.max_sessions).toBe(0);
    });

    it('returns defaults on malformed JSON', () => {
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue('not valid json {{{');

      const config = load();

      expect(config).toEqual(getDefault());
    });

    it('returns defaults on empty file', () => {
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue('');

      const config = load();

      expect(config).toEqual(getDefault());
    });

    it('preserves all valid fields from a complete config file', () => {
      const fullConfig: Config = {
        config_version: 1,
        default_shell: 'bash',
        default_terminal: 'iterm2',
        default_time_range: '7d',
        default_sort: 'created',
        default_sort_order: 'asc',
        default_pivot: 'repo',
        show_preview: false,
        max_sessions: 500,
        yoloMode: true,
        agent: 'coding-agent',
        model: 'claude-sonnet-4',
        custom_command: 'ghcs --resume {sessionId}',
        theme: 'dark',
        workspace_recovery: false,
      };
      existsSyncMock.mockReturnValue(true);
      readFileSyncMock.mockReturnValue(JSON.stringify(fullConfig));

      const loaded = load();

      expect(loaded).toEqual(fullConfig);
    });
  });

  describe('save', () => {
    it('creates config directory if it does not exist', () => {
      existsSyncMock.mockReturnValue(false);

      save(getDefault());

      expect(mkdirSyncMock).toHaveBeenCalledWith(
        expect.any(String),
        { recursive: true, mode: 0o700 },
      );
    });

    it('skips directory creation when it already exists', () => {
      existsSyncMock.mockReturnValue(true);

      save(getDefault());

      expect(mkdirSyncMock).not.toHaveBeenCalled();
    });

    it('writes to a temp file then renames atomically', () => {
      existsSyncMock.mockReturnValue(true);

      save(getDefault());

      expect(writeFileSyncMock).toHaveBeenCalledWith(
        expect.stringContaining('.tmp'),
        expect.any(String),
        { encoding: 'utf-8', mode: 0o600 },
      );
      expect(renameSyncMock).toHaveBeenCalledWith(
        expect.stringContaining('.tmp'),
        expect.stringContaining('config.json'),
      );
    });

    it('serializes config as formatted JSON', () => {
      existsSyncMock.mockReturnValue(true);

      const config = getDefault();
      save(config);

      const written = writeFileSyncMock.mock.calls[0][1] as string;
      expect(JSON.parse(written)).toEqual(config);
      expect(written).toContain('\n'); // Pretty-printed
    });
  });

  describe('getConfigPath', () => {
    it('returns a path ending with config.json', () => {
      const path = getConfigPath();

      expect(path).toMatch(/config\.json$/);
    });
  });
});
