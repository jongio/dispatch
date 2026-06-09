import { spawn, execFile } from 'child_process';
import { getShells, getTerminals } from './shells';
import type { ShellInfo, TerminalInfo } from './shells';

export interface LaunchOptions {
  shell?: string;
  terminal?: string;
  yoloMode?: boolean;
  agent?: string;
  model?: string;
  customCommand?: string;
  paneDirection?: 'horizontal' | 'vertical';
}

export interface LaunchResult {
  success: boolean;
  error?: string;
}

/**
 * LaunchManager handles spawning terminal sessions with the Copilot CLI.
 * All launched processes are detached so they outlive the Dispatch window.
 */
export class LaunchManager {
  private cachedShells: ShellInfo[] | null = null;
  private cachedTerminals: TerminalInfo[] | null = null;

  /**
   * Opens a terminal in the current context (in-place).
   * Spawns the shell directly without a new window/tab.
   */
  launchInPlace(sessionId: string, opts: LaunchOptions = {}): LaunchResult {
    const command = this.buildResumeCommand(sessionId, opts);
    const shell = this.resolveShell(opts.shell);
    if (!shell) {
      return { success: false, error: 'No suitable shell found on this system.' };
    }

    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  /**
   * Opens a new terminal tab with the session.
   */
  launchNewTab(sessionId: string, opts: LaunchOptions = {}): LaunchResult {
    const command = this.buildResumeCommand(sessionId, opts);
    const shell = this.resolveShell(opts.shell);
    if (!shell) {
      return { success: false, error: 'No suitable shell found on this system.' };
    }

    const terminal = this.resolveTerminal(opts.terminal);
    if (!terminal) {
      // Fall back to in-place spawn if no terminal supports tabs
      return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
    }

    return this.launchInTerminalTab(terminal, shell, command, sessionId);
  }

  /**
   * Opens a new terminal window with the session.
   */
  launchNewWindow(sessionId: string, opts: LaunchOptions = {}): LaunchResult {
    const command = this.buildResumeCommand(sessionId, opts);
    const shell = this.resolveShell(opts.shell);
    if (!shell) {
      return { success: false, error: 'No suitable shell found on this system.' };
    }

    const terminal = this.resolveTerminal(opts.terminal);
    if (!terminal) {
      return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
    }

    return this.launchInTerminalWindow(terminal, shell, command, sessionId);
  }

  /**
   * Opens a split pane in the terminal (Windows Terminal, iTerm2, Konsole).
   */
  launchSplitPane(sessionId: string, opts: LaunchOptions = {}): LaunchResult {
    const command = this.buildResumeCommand(sessionId, opts);
    const shell = this.resolveShell(opts.shell);
    if (!shell) {
      return { success: false, error: 'No suitable shell found on this system.' };
    }

    const terminal = this.resolveTerminal(opts.terminal);
    if (!terminal?.supportsSplitPane) {
      return { success: false, error: `Split pane is not supported by ${terminal?.displayName ?? 'the available terminal'}.` };
    }

    return this.launchInSplitPane(terminal, shell, command, opts.paneDirection ?? 'horizontal', sessionId);
  }

  /**
   * Launches multiple sessions in parallel.
   */
  launchMulti(
    sessionIds: string[],
    mode: 'inPlace' | 'newTab' | 'newWindow' | 'splitPane',
    opts: LaunchOptions = {},
  ): LaunchResult[] {
    const launcher = {
      inPlace: (id: string) => this.launchInPlace(id, opts),
      newTab: (id: string) => this.launchNewTab(id, opts),
      newWindow: (id: string) => this.launchNewWindow(id, opts),
      splitPane: (id: string) => this.launchSplitPane(id, opts),
    };

    return sessionIds.map((id) => launcher[mode](id));
  }

  /**
   * Builds the `gh copilot session resume` command string with flags.
   */
  private buildResumeCommand(sessionId: string, opts: LaunchOptions): string {
    if (opts.customCommand) {
      return opts.customCommand;
    }

    const parts = ['gh', 'copilot', 'session', 'resume', sessionId];

    if (opts.yoloMode) {
      parts.push('--yolo');
    }
    if (opts.agent) {
      parts.push('--agent', opts.agent);
    }
    if (opts.model) {
      parts.push('--model', opts.model);
    }

    return parts.join(' ');
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
   * Uses -NoExit (pwsh) or keeps stdin open so the interactive session stays alive.
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
        // bash, zsh, fish, sh, git-bash — use -i for interactive
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

  private launchInTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    sessionId: string,
  ): LaunchResult {
    const title = `Dispatch: ${sessionId.slice(0, 8)}`;

    switch (process.platform) {
      case 'win32':
        return this.launchWindowsTerminalTab(terminal, shell, command, title);
      case 'darwin':
        return this.launchMacTerminalTab(terminal, shell, command, title);
      default:
        return this.launchLinuxTerminalTab(terminal, shell, command);
    }
  }

  private launchInTerminalWindow(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    sessionId: string,
  ): LaunchResult {
    const title = `Dispatch: ${sessionId.slice(0, 8)}`;

    switch (process.platform) {
      case 'win32':
        return this.launchWindowsTerminalWindow(terminal, shell, command, title);
      case 'darwin':
        return this.launchMacTerminalWindow(terminal, shell, command, title);
      default:
        return this.launchLinuxTerminalWindow(terminal, shell, command, title);
    }
  }

  private launchInSplitPane(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    direction: 'horizontal' | 'vertical',
    sessionId: string,
  ): LaunchResult {
    switch (process.platform) {
      case 'win32':
        return this.launchWindowsTerminalSplitPane(terminal, shell, command, direction);
      case 'darwin':
        return this.launchMacSplitPane(terminal, shell, command, direction);
      default:
        return this.launchLinuxSplitPane(terminal, shell, command, direction);
    }
  }

  // ─── Windows Terminal ──────────────────────────────────────────────

  private launchWindowsTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    title: string,
  ): LaunchResult {
    if (terminal.name === 'windows-terminal') {
      // wt.exe -w 0 new-tab --title "title" pwsh.exe -NoExit -Command "command"
      const shellArgs = this.buildShellArgs(shell, command);
      const args = ['-w', '0', 'new-tab', '--title', title, shell.path, ...shellArgs];
      return this.spawnDetached(terminal.path, args);
    }

    // Fallback: open in new console window
    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  private launchWindowsTerminalWindow(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    title: string,
  ): LaunchResult {
    if (terminal.name === 'windows-terminal') {
      // wt.exe new-tab --title "Dispatch: {id}" {shell} -c "{command}"
      const args = ['new-tab', '--title', title, shell.path, ...this.buildShellArgs(shell, command)];
      return this.spawnDetached(terminal.path, args);
    }

    // Generic: spawn the shell directly (opens a new console window on Windows)
    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  private launchWindowsTerminalSplitPane(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    direction: 'horizontal' | 'vertical',
  ): LaunchResult {
    if (terminal.name !== 'windows-terminal') {
      return { success: false, error: `${terminal.displayName} does not support split panes.` };
    }

    // wt.exe -w 0 split-pane --size 0.5 -H|-V {shell} -c "{command}"
    const dirFlag = direction === 'horizontal' ? '-H' : '-V';
    const args = ['-w', '0', 'split-pane', '--size', '0.5', dirFlag, shell.path, ...this.buildShellArgs(shell, command)];
    return this.spawnDetached(terminal.path, args);
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
          do script "${escapeAppleScript(this.buildFullShellCommand(shell, command))}" in front window
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
              write text "${escapeAppleScript(this.buildFullShellCommand(shell, command))}"
            end tell
          end tell
        end tell
      `);
    }

    // Generic: spawn directly
    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  private launchMacTerminalWindow(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    title: string,
  ): LaunchResult {
    if (terminal.name === 'terminal') {
      return this.runAppleScript(`
        tell application "Terminal"
          activate
          do script "${escapeAppleScript(this.buildFullShellCommand(shell, command))}"
          set custom title of front window to "${escapeAppleScript(title)}"
        end tell
      `);
    }

    if (terminal.name === 'iterm2') {
      return this.runAppleScript(`
        tell application "iTerm"
          activate
          create window with default profile
          tell current session of current window
            write text "${escapeAppleScript(this.buildFullShellCommand(shell, command))}"
          end tell
        end tell
      `);
    }

    // Alacritty, Kitty, Warp: spawn with -e flag
    if (terminal.name === 'alacritty') {
      return this.spawnDetached(terminal.path, ['-e', shell.path, ...this.buildShellArgs(shell, command)]);
    }

    if (terminal.name === 'kitty') {
      return this.spawnDetached(terminal.path, ['--', shell.path, ...this.buildShellArgs(shell, command)]);
    }

    return this.spawnDetached(shell.path, this.buildShellArgs(shell, command));
  }

  private launchMacSplitPane(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    direction: 'horizontal' | 'vertical',
  ): LaunchResult {
    if (terminal.name === 'iterm2') {
      const splitCmd = direction === 'horizontal' ? 'split horizontally' : 'split vertically';
      return this.runAppleScript(`
        tell application "iTerm"
          activate
          tell current session of current window
            ${splitCmd} with default profile
          end tell
          tell current session of current window
            write text "${escapeAppleScript(this.buildFullShellCommand(shell, command))}"
          end tell
        end tell
      `);
    }

    return { success: false, error: `${terminal.displayName} does not support split panes on macOS.` };
  }

  // ─── Linux ─────────────────────────────────────────────────────────

  private launchLinuxTerminalTab(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
  ): LaunchResult {
    const fullCommand = this.buildFullShellCommand(shell, command);

    switch (terminal.name) {
      case 'gnome-terminal':
        return this.spawnDetached(terminal.path, ['--tab', '--', shell.path, '-c', fullCommand]);
      case 'konsole':
        return this.spawnDetached(terminal.path, ['--new-tab', '-e', shell.path, '-c', fullCommand]);
      case 'kitty':
        return this.spawnDetached(terminal.path, ['@', 'new-window', '--', shell.path, '-c', fullCommand]);
      default:
        return this.spawnDetached(terminal.path, ['-e', shell.path, '-c', fullCommand]);
    }
  }

  private launchLinuxTerminalWindow(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    title: string,
  ): LaunchResult {
    const fullCommand = this.buildFullShellCommand(shell, command);

    switch (terminal.name) {
      case 'gnome-terminal':
        return this.spawnDetached(terminal.path, ['--title', title, '--', shell.path, '-c', fullCommand]);
      case 'konsole':
        return this.spawnDetached(terminal.path, ['-e', shell.path, '-c', fullCommand]);
      case 'alacritty':
        return this.spawnDetached(terminal.path, ['--title', title, '-e', shell.path, '-c', fullCommand]);
      case 'kitty':
        return this.spawnDetached(terminal.path, ['--title', title, '--', shell.path, '-c', fullCommand]);
      case 'xterm':
        return this.spawnDetached(terminal.path, ['-T', title, '-e', shell.path, '-c', fullCommand]);
      default:
        return this.spawnDetached(terminal.path, ['-e', shell.path, '-c', fullCommand]);
    }
  }

  private launchLinuxSplitPane(
    terminal: TerminalInfo,
    shell: ShellInfo,
    command: string,
    direction: 'horizontal' | 'vertical',
  ): LaunchResult {
    if (terminal.name === 'konsole') {
      const splitArg = direction === 'horizontal' ? '--split=left-right' : '--split=top-bottom';
      const fullCommand = this.buildFullShellCommand(shell, command);
      return this.spawnDetached(terminal.path, [splitArg, '-e', shell.path, '-c', fullCommand]);
    }

    return { success: false, error: `${terminal.displayName} does not support split panes on Linux.` };
  }

  // ─── Helpers ───────────────────────────────────────────────────────

  /**
   * Builds a full command string for shells that need it (e.g., when embedding
   * in AppleScript or passing to a terminal emulator).
   */
  private buildFullShellCommand(shell: ShellInfo, command: string): string {
    switch (shell.name) {
      case 'fish':
        return command;
      default:
        return command;
    }
  }

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
