import { execFileSync } from 'child_process';
import { existsSync } from 'fs';
import { join } from 'path';
import { homedir } from 'os';

export interface ShellInfo {
  name: string;
  path: string;
  displayName: string;
  isDefault: boolean;
}

export interface TerminalInfo {
  name: string;
  path: string;
  displayName: string;
  isDefault: boolean;
  supportsNewTab: boolean;
  supportsSplitPane: boolean;
}

/**
 * Detects installed shells available on the current system.
 * Returns an array sorted by preference (default first).
 */
export function getShells(): ShellInfo[] {
  switch (process.platform) {
    case 'win32':
      return detectWindowsShells();
    case 'darwin':
      return detectMacShells();
    default:
      return detectLinuxShells();
  }
}

/**
 * Detects available terminal emulators on the current system.
 * Returns an array sorted by preference (default first).
 */
export function getTerminals(): TerminalInfo[] {
  switch (process.platform) {
    case 'win32':
      return detectWindowsTerminals();
    case 'darwin':
      return detectMacTerminals();
    default:
      return detectLinuxTerminals();
  }
}

function detectWindowsShells(): ShellInfo[] {
  const shells: ShellInfo[] = [];
  const defaultShell = process.env.COMSPEC ?? 'C:\\Windows\\System32\\cmd.exe';

  // PowerShell 7 (pwsh)
  const pwshPaths = [
    join(process.env.ProgramFiles ?? 'C:\\Program Files', 'PowerShell', '7', 'pwsh.exe'),
    join(process.env['ProgramFiles(x86)'] ?? 'C:\\Program Files (x86)', 'PowerShell', '7', 'pwsh.exe'),
  ];
  const pwshPath = pwshPaths.find(existsSync) ?? findOnPath('pwsh.exe');
  if (pwshPath) {
    shells.push({
      name: 'pwsh',
      path: pwshPath,
      displayName: 'PowerShell 7',
      isDefault: false,
    });
  }

  // PowerShell 5 (Windows PowerShell)
  const powershellPath = join(
    process.env.SystemRoot ?? 'C:\\Windows',
    'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe',
  );
  if (existsSync(powershellPath)) {
    shells.push({
      name: 'powershell',
      path: powershellPath,
      displayName: 'Windows PowerShell',
      isDefault: false,
    });
  }

  // cmd.exe
  const cmdPath = join(process.env.SystemRoot ?? 'C:\\Windows', 'System32', 'cmd.exe');
  if (existsSync(cmdPath)) {
    shells.push({
      name: 'cmd',
      path: cmdPath,
      displayName: 'Command Prompt',
      isDefault: cmdPath.toLowerCase() === defaultShell.toLowerCase(),
    });
  }

  // Git Bash
  const gitBashPaths = [
    join(process.env.ProgramFiles ?? 'C:\\Program Files', 'Git', 'bin', 'bash.exe'),
    join(process.env['ProgramFiles(x86)'] ?? 'C:\\Program Files (x86)', 'Git', 'bin', 'bash.exe'),
    join(process.env.LOCALAPPDATA ?? '', 'Programs', 'Git', 'bin', 'bash.exe'),
  ];
  const gitBashPath = gitBashPaths.find(existsSync);
  if (gitBashPath) {
    shells.push({
      name: 'git-bash',
      path: gitBashPath,
      displayName: 'Git Bash',
      isDefault: false,
    });
  }

  // WSL bash
  const wslPath = join(process.env.SystemRoot ?? 'C:\\Windows', 'System32', 'wsl.exe');
  if (existsSync(wslPath)) {
    shells.push({
      name: 'wsl',
      path: wslPath,
      displayName: 'WSL (bash)',
      isDefault: false,
    });
  }

  // Mark pwsh as default if nothing else is default (prefer modern PowerShell)
  if (shells.length > 0 && !shells.some((s) => s.isDefault)) {
    const preferredDefault = shells.find((s) => s.name === 'pwsh') ?? shells[0];
    preferredDefault.isDefault = true;
  }

  return sortDefaultFirst(shells);
}

function detectMacShells(): ShellInfo[] {
  const shells: ShellInfo[] = [];
  const defaultShell = process.env.SHELL ?? '/bin/zsh';

  const candidates: Array<{ name: string; paths: string[]; displayName: string }> = [
    { name: 'zsh', paths: ['/bin/zsh', '/usr/local/bin/zsh', '/opt/homebrew/bin/zsh'], displayName: 'Zsh' },
    { name: 'bash', paths: ['/bin/bash', '/usr/local/bin/bash', '/opt/homebrew/bin/bash'], displayName: 'Bash' },
    { name: 'fish', paths: ['/usr/local/bin/fish', '/opt/homebrew/bin/fish'], displayName: 'Fish' },
    { name: 'sh', paths: ['/bin/sh'], displayName: 'POSIX Shell' },
  ];

  for (const candidate of candidates) {
    const foundPath = candidate.paths.find(existsSync);
    if (foundPath) {
      shells.push({
        name: candidate.name,
        path: foundPath,
        displayName: candidate.displayName,
        isDefault: foundPath === defaultShell,
      });
    }
  }

  if (shells.length > 0 && !shells.some((s) => s.isDefault)) {
    shells[0].isDefault = true;
  }

  return sortDefaultFirst(shells);
}

function detectLinuxShells(): ShellInfo[] {
  const shells: ShellInfo[] = [];
  const defaultShell = process.env.SHELL ?? '/bin/bash';

  const candidates: Array<{ name: string; paths: string[]; displayName: string }> = [
    { name: 'bash', paths: ['/bin/bash', '/usr/bin/bash'], displayName: 'Bash' },
    { name: 'zsh', paths: ['/bin/zsh', '/usr/bin/zsh', '/usr/local/bin/zsh'], displayName: 'Zsh' },
    { name: 'fish', paths: ['/usr/bin/fish', '/usr/local/bin/fish'], displayName: 'Fish' },
    { name: 'sh', paths: ['/bin/sh'], displayName: 'POSIX Shell' },
  ];

  for (const candidate of candidates) {
    const foundPath = candidate.paths.find(existsSync);
    if (foundPath) {
      shells.push({
        name: candidate.name,
        path: foundPath,
        displayName: candidate.displayName,
        isDefault: foundPath === defaultShell,
      });
    }
  }

  if (shells.length > 0 && !shells.some((s) => s.isDefault)) {
    shells[0].isDefault = true;
  }

  return sortDefaultFirst(shells);
}

function detectWindowsTerminals(): TerminalInfo[] {
  const terminals: TerminalInfo[] = [];

  // Windows Terminal (wt.exe)
  const wtPath = findOnPath('wt.exe');
  if (wtPath) {
    terminals.push({
      name: 'windows-terminal',
      path: wtPath,
      displayName: 'Windows Terminal',
      isDefault: true,
      supportsNewTab: true,
      supportsSplitPane: true,
    });
  }

  // cmd.exe (as a terminal host)
  const cmdPath = join(process.env.SystemRoot ?? 'C:\\Windows', 'System32', 'cmd.exe');
  if (existsSync(cmdPath)) {
    terminals.push({
      name: 'cmd',
      path: cmdPath,
      displayName: 'Command Prompt Window',
      isDefault: !wtPath,
      supportsNewTab: false,
      supportsSplitPane: false,
    });
  }

  // PowerShell window
  const powershellPath = join(
    process.env.SystemRoot ?? 'C:\\Windows',
    'System32', 'WindowsPowerShell', 'v1.0', 'powershell.exe',
  );
  if (existsSync(powershellPath)) {
    terminals.push({
      name: 'powershell-window',
      path: powershellPath,
      displayName: 'PowerShell Window',
      isDefault: false,
      supportsNewTab: false,
      supportsSplitPane: false,
    });
  }

  return sortDefaultFirst(terminals);
}

function detectMacTerminals(): TerminalInfo[] {
  const terminals: TerminalInfo[] = [];

  // Terminal.app (always available on macOS)
  terminals.push({
    name: 'terminal',
    path: '/System/Applications/Utilities/Terminal.app',
    displayName: 'Terminal',
    isDefault: true,
    supportsNewTab: true,
    supportsSplitPane: false,
  });

  // iTerm2
  const itermPaths = [
    '/Applications/iTerm.app',
    join(homedir(), 'Applications', 'iTerm.app'),
  ];
  const itermPath = itermPaths.find(existsSync);
  if (itermPath) {
    terminals.push({
      name: 'iterm2',
      path: itermPath,
      displayName: 'iTerm2',
      isDefault: false,
      supportsNewTab: true,
      supportsSplitPane: true,
    });
  }

  // Alacritty
  const alacrittyPath = findOnPath('alacritty');
  if (alacrittyPath) {
    terminals.push({
      name: 'alacritty',
      path: alacrittyPath,
      displayName: 'Alacritty',
      isDefault: false,
      supportsNewTab: false,
      supportsSplitPane: false,
    });
  }

  // Warp
  if (existsSync('/Applications/Warp.app')) {
    terminals.push({
      name: 'warp',
      path: '/Applications/Warp.app',
      displayName: 'Warp',
      isDefault: false,
      supportsNewTab: true,
      supportsSplitPane: true,
    });
  }

  // Kitty
  const kittyPath = findOnPath('kitty');
  if (kittyPath) {
    terminals.push({
      name: 'kitty',
      path: kittyPath,
      displayName: 'Kitty',
      isDefault: false,
      supportsNewTab: true,
      supportsSplitPane: false,
    });
  }

  return terminals;
}

function detectLinuxTerminals(): TerminalInfo[] {
  const terminals: TerminalInfo[] = [];

  const candidates: Array<{
    name: string;
    binary: string;
    displayName: string;
    supportsNewTab: boolean;
    supportsSplitPane: boolean;
  }> = [
    { name: 'gnome-terminal', binary: 'gnome-terminal', displayName: 'GNOME Terminal', supportsNewTab: true, supportsSplitPane: false },
    { name: 'konsole', binary: 'konsole', displayName: 'Konsole', supportsNewTab: true, supportsSplitPane: true },
    { name: 'alacritty', binary: 'alacritty', displayName: 'Alacritty', supportsNewTab: false, supportsSplitPane: false },
    { name: 'kitty', binary: 'kitty', displayName: 'Kitty', supportsNewTab: true, supportsSplitPane: false },
    { name: 'xterm', binary: 'xterm', displayName: 'XTerm', supportsNewTab: false, supportsSplitPane: false },
  ];

  let foundDefault = false;
  for (const candidate of candidates) {
    const binPath = findOnPath(candidate.binary);
    if (binPath) {
      terminals.push({
        name: candidate.name,
        path: binPath,
        displayName: candidate.displayName,
        isDefault: !foundDefault,
        supportsNewTab: candidate.supportsNewTab,
        supportsSplitPane: candidate.supportsSplitPane,
      });
      foundDefault = true;
    }
  }

  return terminals;
}

/**
 * Attempts to find a binary on the system PATH.
 * Returns the full path or null if not found.
 */
function findOnPath(binary: string): string | null {
  try {
    const cmd = process.platform === 'win32' ? 'where.exe' : 'which';
    const result = execFileSync(cmd, [binary], {
      encoding: 'utf8',
      timeout: 3000,
      windowsHide: true,
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    const firstLine = result.trim().split('\n')[0]?.trim();
    return firstLine && existsSync(firstLine) ? firstLine : null;
  } catch {
    return null;
  }
}

function sortDefaultFirst<T extends { isDefault: boolean }>(items: T[]): T[] {
  return items.sort((a, b) => {
    if (a.isDefault && !b.isDefault) return -1;
    if (!a.isDefault && b.isDefault) return 1;
    return 0;
  });
}
