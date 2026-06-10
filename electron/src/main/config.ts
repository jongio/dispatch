import { app, shell } from 'electron';
import { existsSync, mkdirSync, readFileSync, renameSync, writeFileSync } from 'fs';
import { dirname, join } from 'path';
import { homedir } from 'os';
import { randomBytes } from 'crypto';

export interface Config {
  config_version?: number;
  default_shell: string;
  default_terminal: string;
  default_time_range: string;
  default_sort: string;
  default_sort_order: string;
  default_pivot: string;
  show_preview: boolean;
  max_sessions: number;
  yoloMode: boolean;
  agent: string;
  model: string;
  custom_command: string;
  theme: string;
  workspace_recovery: boolean;
}

const MAX_MAX_SESSIONS = 10_000;

function getConfigDir(): string {
  switch (process.platform) {
    case 'win32':
      return join(process.env.APPDATA || join(homedir(), 'AppData', 'Roaming'), 'dispatch');
    case 'darwin':
      return join(homedir(), 'Library', 'Application Support', 'dispatch');
    default:
      return join(process.env.XDG_CONFIG_HOME || join(homedir(), '.config'), 'dispatch');
  }
}

export function getConfigPath(): string {
  return join(getConfigDir(), 'config.json');
}

export function getDefault(): Config {
  return {
    config_version: 1,
    default_shell: '',
    default_terminal: '',
    default_time_range: '1d',
    default_sort: 'updated',
    default_sort_order: 'desc',
    default_pivot: 'folder',
    show_preview: true,
    max_sessions: 100,
    yoloMode: false,
    agent: '',
    model: '',
    custom_command: '',
    theme: '',
    workspace_recovery: true,
  };
}

export function load(): Config {
  const path = getConfigPath();

  if (!existsSync(path)) {
    return getDefault();
  }

  try {
    const data = readFileSync(path, 'utf-8');
    const parsed = JSON.parse(data) as Partial<Config>;
    const defaults = getDefault();

    // Merge parsed values over defaults so missing keys retain default values
    const config: Config = { ...defaults, ...parsed };

    // Clamp max_sessions
    if (config.max_sessions > MAX_MAX_SESSIONS) {
      config.max_sessions = MAX_MAX_SESSIONS;
    }
    if (config.max_sessions < 0) {
      config.max_sessions = 0;
    }

    return config;
  } catch {
    // If the file is corrupted, return defaults
    return getDefault();
  }
}

export function save(config: Config): void {
  const path = getConfigPath();
  const dir = dirname(path);

  // Ensure the config directory exists
  if (!existsSync(dir)) {
    mkdirSync(dir, { recursive: true, mode: 0o700 });
  }

  const data = JSON.stringify(config, null, '  ');

  // Atomic write: write to temp file, then rename
  const tmpPath = `${path}.${randomBytes(4).toString('hex')}.tmp`;
  writeFileSync(tmpPath, data, { encoding: 'utf-8', mode: 0o600 });
  renameSync(tmpPath, path);
}

export function openConfigDirectory(): void {
  const dir = getConfigDir();
  shell.openPath(dir);
}
