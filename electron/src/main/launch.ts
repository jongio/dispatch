import { spawn, execFile, execSync } from 'child_process';
import { getShells, getTerminals } from './shells';
import type { ShellInfo, TerminalInfo } from './shells';

/**
 * The static Windows Terminal window name. All sessions launched from Electron
 * reuse this named window, opening each session as a new tab within it.
 */
const WT_WINDOW_NAME = 'dispatch';

/**
 * Strict pattern for valid session IDs. Only alphanumeric, dots, hyphens,
 * and underscores allowed. Prevents shell injection via session ID.
 */
const SESSION_ID_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._-]{0,127}$/;

/**
 * Pattern for safe CLI argument values (agent name, model name).
 * Allows alphanumeric, hyphens, dots, underscores, and forward slashes.
 */
const SAFE_ARG_PATTERN = /^[a-zA-Z0-9][a-zA-Z0-9._\-/]{0,127}$/;

export interface LaunchOptions {
  shell?: string;
  terminal?: string;
  yoloMode?: boolean;
  agent?: string;
  model?: string;
  customCommand?: string;
}

export interface LaunchResult {
  success: boolean;
  error?: string;
}

/**
 * LaunchManager handles spawning terminal sessions with the Copilot CLI.
 *
 * From Electron, the launch model is simple: always open a new tab in the
 * named "dispatch" Windows Terminal window. If the window doesn't exist yet,
 * Windows Terminal creates it automatically.
 */
export class LaunchManager {
  private cachedShells: ShellInfo[] | null = null;
  private cachedTerminals: TerminalInfo[] | null = null;
  private cachedCliBinary: string | null = null;

  /**
   * Launches a session as a new tab in the "dispatch" Windows Terminal window.
   */
  launch(sessionId: string, opts: LaunchOptions = {}): LaunchResult {
    console.log('[LaunchManager.launch] sessionId:', sessionId);
    const validationError = this.validateInputs(sessionId, opts);
    if (validationError) {
      console.log('[LaunchManager.launch] VALIDATION FAILED:', validationError);
      return { success: false, error: validationError };
    }

    const command = this.buildResumeCommand(sessionId, opts);
    const shell = this.resolveShell(opts.shell);
    if (!shell) {
      console.log('[LaunchManager.launch] NO SHELL FOUND');
      return { success: false, error: 'No suitable shell found on this system.' };
    }

    const terminal = this.resolveTerminal(opts.terminal);
    console.log('[LaunchManager.launch] terminal:', terminal?.name, 'shell:', shell.name);

    if (terminal?.name === 'windows-terminal') {
      return this.launchInWindowsTerminal(terminal, shell, command, sessionId);
    }

    // Fallback for non-WT platforms: open in whatever terminal is available
    if (terminal) {
      return this.launchInTerminalTab(terminal, shell, command, sessionId);
    }

    // Last resort: spawn shell directly
    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  /**
   * Launches multiple sessions, each as a new tab.
   * Staggers spawns slightly to avoid WT race conditions with named windows.
   */
  launchMulti(sessionIds: string[], opts: LaunchOptions = {}): LaunchResult[] {
    const results: LaunchResult[] = [];
    for (let i = 0; i < sessionIds.length; i++) {
      if (i > 0) {
        const start = Date.now();
        while (Date.now() - start < 500) { /* wait for WT to register the window */ }
      }
      results.push(this.launch(sessionIds[i], opts));
    }
    return results;
  }

  /**
   * Validates all user-supplied inputs before command construction.
   * Returns an error message if validation fails, or null if valid.
   */
  private validateInputs(sessionId: string, opts: LaunchOptions): string | null {
    if (!SESSION_ID_PATTERN.test(sessionId)) {
      return `Invalid session ID: must match ${SESSION_ID_PATTERN.source}`;
    }
    if (opts.agent && !SAFE_ARG_PATTERN.test(opts.agent)) {
      return 'Invalid agent name: contains disallowed characters.';
    }
    if (opts.model && !SAFE_ARG_PATTERN.test(opts.model)) {
      return 'Invalid model name: contains disallowed characters.';
    }
    if (opts.customCommand && opts.customCommand.length > 1024) {
      return 'Custom command exceeds maximum length (1024 chars).';
    }
    return null;
  }

  /**
   * Builds the resume command string using the correct Copilot CLI binary.
   * Prefers "ghcs" (Copilot CLI standalone), falls back to "copilot".
   */
  private buildResumeCommand(sessionId: string, opts: LaunchOptions): string {
    if (opts.customCommand) {
      return opts.customCommand.replace('{sessionId}', sessionId);
    }

    const binary = this.findCliBinary();
    const parts = [binary, '--resume', sessionId];

    if (opts.yoloMode) {
      parts.push('--allow-all');
    }
    if (opts.agent) {
      parts.push('--agent', opts.agent);
    }
    if (opts.model) {
      parts.push('--model', opts.model);
    }

    return parts.join(' ');
  }

  /**
   * Returns the Copilot CLI binary name. Checks PATH first, then well-known
   * install locations (Electron often has a stripped PATH).
   */
  private findCliBinary(): string {
    if (this.cachedCliBinary) return this.cachedCliBinary;

    // Try PATH lookup
    try {
      const cmd = process.platform === 'win32' ? 'where copilot' : 'which copilot';
      const result = execSync(cmd, { encoding: 'utf-8', timeout: 3000 }).trim();
      if (result) {
        this.cachedCliBinary = 'copilot';
        return 'copilot';
      }
    } catch {
      // not on PATH
    }

    // Check well-known install paths (Electron often has a stripped PATH)
    const { existsSync } = require('fs');
    const { join } = require('path');
    const candidates = process.platform === 'win32'
      ? [
          join(process.env.ProgramFiles || 'C:\\Program Files', 'nodejs', 'copilot.cmd'),
          join(process.env.LOCALAPPDATA || '', 'npm', 'copilot.cmd'),
          join(process.env.APPDATA || '', 'npm', 'copilot.cmd'),
        ]
      : ['/usr/local/bin/copilot', '/usr/bin/copilot'];

    for (const p of candidates) {
      if (p && existsSync(p)) {
        this.cachedCliBinary = p;
        return p;
      }
    }

    this.cachedCliBinary = 'copilot';
    return 'copilot';
  }

  private resolveShell(shellName?: string): ShellInfo | null {
    const shells = this.getShellsCached();
    if (shells.length === 0) return null;

    if (shellName) {
      return shells.find((s) => s.name === shellName) ?? null;
    }

    return shells.find((s) => s.isDefault) ?? shells[0];
  }

  private resolveTerminal(terminalName?: string): TerminalInfo | null {
    const terminals = this.getTerminalsCached();
    if (terminals.length === 0) return null;

    if (terminalName) {
      return terminals.find((t) => t.name === terminalName) ?? null;
    }

    return terminals.find((t) => t.isDefault) ?? terminals[0];
  }

  private getShellsCached(): ShellInfo[] {
    if (!this.cachedShells) {
      this.cachedShells = getShells();
    }
    return this.cachedShells;
  }

  private getTerminalsCached(): TerminalInfo[] {
    if (!this.cachedTerminals) {
      this.cachedTerminals = getTerminals();
    }
    return this.cachedTerminals;
  }

  /**
   * Builds the argument array for executing a command within a given shell.
   */
  private buildShellArgs(shell: ShellInfo, command: string): string[] {
    switch (shell.name) {
      case 'cmd':
        return ['/k', command];
      case 'pwsh':
      case 'powershell':
        return ['-NoExit', '-Command', command];
      case 'wsl':
        return ['--', 'bash', '-ic', command];
      default:
        // bash, zsh, fish, sh, git-bash
        return ['-ic', command];
    }
  }

  /**
   * Spawns a detached process that outlives the Dispatch app.
   */
  private spawnDetached(binary: string, args: string[]): LaunchResult {
    try {
      const child = spawn(binary, args, {
        detached: true,
        stdio: 'ignore',
        windowsHide: false,
      });
      child.unref();
      return { success: true };
    } catch (err) {
      return { success: false, error: formatSpawnError(err) };
    }
  }

  /**
   * Opens a new tab in the named "dispatch" Windows Terminal window.
   * Uses bare 'wt.exe' name — Node resolves it via PATH, which handles
   * Windows App Execution Aliases correctly (unlike full reparse-point paths).
   */
  private launchInWindowsTerminal(
    _terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    sessionId: string,
  ): LaunchResult {
    const title = `Dispatch: ${sessionId.slice(0, 8)}`;
    // Use bare exe names — Node's spawn resolves via PATH+PATHEXT
    const shellExe = shell.name === 'cmd' ? 'cmd.exe'
      : shell.name === 'powershell' ? 'powershell.exe'
      : 'pwsh.exe';
    const args = ['-w', WT_WINDOW_NAME, 'new-tab', '--title', title, shellExe];
    if (shell.name === 'cmd') {
      args.push('/k', command);
    } else {
      args.push('-NoExit', '-Command', command);
    }
    return this.spawnDetached('wt.exe', args);
  }

  /**
   * Fallback for non-Windows platforms: open a new tab in the detected terminal.
   */
  private launchInTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    sessionId: string,
  ): LaunchResult {
    const title = `Dispatch: ${sessionId.slice(0, 8)}`;

    switch (process.platform) {
      case 'darwin':
        return this.launchMacTerminalTab(terminal, shell, command, title);
      default:
        return this.launchLinuxTerminalTab(terminal, shell, command);
    }
  }

  // ─── macOS ─────────────────────────────────────────────────────────

  private launchMacTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    title: string,
  ): LaunchResult {
    if (terminal.name === 'terminal') {
      return this.runAppleScript(`
        tell application "Terminal"
          activate
          tell application "System Events" to keystroke "t" using command down
          delay 0.3
          do script "${escapeAppleScript(command)}" in front window
          set custom title of front window to "${escapeAppleScript(title)}"
        end tell
      `);
    }

    if (terminal.name === 'iterm2') {
      return this.runAppleScript(`
        tell application "iTerm"
          activate
          tell current window
            create tab with default profile
            tell current session of current tab
              write text "${escapeAppleScript(command)}"
            end tell
          end tell
        end tell
      `);
    }

    // Alacritty, Kitty, etc.
    if (terminal.name === 'alacritty') {
      return this.spawnDetached(terminal.path, ['-e', shell.path, ...this.buildShellArgs(shell, command)]);
    }

    if (terminal.name === 'kitty') {
      return this.spawnDetached(terminal.path, ['--', shell.path, ...this.buildShellArgs(shell, command)]);
    }

    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  // ─── Linux ─────────────────────────────────────────────────────────

  private launchLinuxTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
  ): LaunchResult {
    switch (terminal.name) {
      case 'gnome-terminal':
        return this.spawnDetached(terminal.path, ['--tab', '--', shell.path, '-c', command]);
      case 'konsole':
        return this.spawnDetached(terminal.path, ['--new-tab', '-e', shell.path, '-c', command]);
      case 'kitty':
        return this.spawnDetached(terminal.path, ['@', 'new-window', '--', shell.path, '-c', command]);
      default:
        return this.spawnDetached(terminal.path, ['-e', shell.path, '-c', command]);
    }
  }

  // ─── Helpers ───────────────────────────────────────────────────────

  private runAppleScript(script: string): LaunchResult {
    try {
      execFile('osascript', ['-e', script.trim()], { timeout: 5000 });
      return { success: true };
    } catch (err) {
      return { success: false, error: formatSpawnError(err) };
    }
  }
}

/**
 * Escapes a string for embedding in AppleScript double-quoted strings.
 */
function escapeAppleScript(str: string): string {
  return str.replace(/\\/g, '\\\\').replace(/"/g, '\\"');
}

/**
 * Formats a spawn error into a user-friendly message.
 */
function formatSpawnError(err: unknown): string {
  if (err instanceof Error) {
    const code = (err as NodeJS.ErrnoException).code;
    switch (code) {
      case 'ENOENT':
        return 'Executable not found. Make sure the shell or terminal is installed and on your PATH.';
      case 'EACCES':
        return 'Permission denied. Check that you have execute permissions for the target shell.';
      case 'EPERM':
        return 'Operation not permitted. The system blocked launching this process.';
      default:
        return err.message;
    }
  }
  return 'An unknown error occurred while launching the terminal.';
}

