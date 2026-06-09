import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSessionStore } from '../stores/sessionStore';
import { useThemeStore } from '../stores/themeStore';
import type { Config } from '../../preload/index';

const LAUNCH_MODES = [
  { value: 'in-place', label: 'In Place' },
  { value: 'tab', label: 'New Tab' },
  { value: 'window', label: 'New Window' },
  { value: 'pane', label: 'Split Pane' },
];

const PANE_DIRECTIONS = [
  { value: 'auto', label: 'Auto' },
  { value: 'right', label: 'Right' },
  { value: 'down', label: 'Down' },
  { value: 'left', label: 'Left' },
  { value: 'up', label: 'Up' },
];

function getDefaultConfig(): Config {
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
    launch_mode: 'tab',
    pane_direction: 'auto',
    custom_command: '',
    theme: '',
    workspace_recovery: true,
  };
}

export function SettingsModal() {
  const { showSettings, toggleSettings } = useSessionStore();
  const { themes: themeDefinitions, currentTheme, setTheme } = useThemeStore();
  const [config, setConfig] = useState<Config>(getDefaultConfig());
  const [configPath, setConfigPath] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const overlayRef = useRef<HTMLDivElement>(null);
  const firstInputRef = useRef<HTMLInputElement>(null);

  // Build theme options from the theme store
  const themeOptions = [
    { value: 'auto', label: 'Auto (System)' },
    ...themeDefinitions.map((t) => ({ value: t.id, label: t.name })),
  ];

  // Load config when modal opens
  useEffect(() => {
    if (!showSettings) return;

    async function loadConfig() {
      const [cfg, path] = await Promise.all([
        window.dispatch.config.get(),
        window.dispatch.config.getPath(),
      ]);
      setConfig(cfg);
      setConfigPath(path);
    }
    loadConfig();
  }, [showSettings]);

  // Focus first interactive element on open
  useEffect(() => {
    if (showSettings) {
      const timer = setTimeout(() => {
        firstInputRef.current?.focus();
      }, 50);
      return () => clearTimeout(timer);
    }
  }, [showSettings]);

  const handleSave = useCallback(async () => {
    setIsSaving(true);
    try {
      // Sync theme selection to the config before saving
      const configToSave = { ...config, theme: currentTheme === 'auto' ? '' : currentTheme };
      await window.dispatch.config.set(configToSave);
      toggleSettings();
    } finally {
      setIsSaving(false);
    }
  }, [config, currentTheme, toggleSettings]);

  const handleKeyDown = useCallback((e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      e.stopPropagation();
      toggleSettings();
    } else if (e.key === 'Enter' && (e.ctrlKey || e.metaKey)) {
      e.preventDefault();
      handleSave();
    }
  }, [toggleSettings, handleSave]);

  const handleBackdropClick = useCallback((e: React.MouseEvent) => {
    if (e.target === overlayRef.current) {
      toggleSettings();
    }
  }, [toggleSettings]);

  const updateField = useCallback(<K extends keyof Config>(field: K, value: Config[K]) => {
    setConfig((prev) => ({ ...prev, [field]: value }));
  }, []);

  const resetField = useCallback(<K extends keyof Config>(field: K) => {
    const defaults = getDefaultConfig();
    setConfig((prev) => ({ ...prev, [field]: defaults[field] }));
  }, []);

  const handleThemeChange = useCallback((value: string) => {
    // Apply theme immediately for live preview
    const themeId = value === 'auto' ? 'auto' : value;
    setTheme(themeId);
    setConfig((prev) => ({ ...prev, theme: value === 'auto' ? '' : value }));
  }, [setTheme]);

  const handleResetTheme = useCallback(() => {
    setTheme('auto');
    setConfig((prev) => ({ ...prev, theme: '' }));
  }, [setTheme]);

  const handleOpenConfigDir = useCallback(() => {
    window.dispatch.config.openInExplorer();
  }, []);

  if (!showSettings) return null;

  // Determine theme dropdown value
  const themeValue = currentTheme === 'auto' ? 'auto' : currentTheme;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={handleBackdropClick}
      onKeyDown={handleKeyDown}
    >
      <div
        className="flex flex-col w-[600px] max-h-[85vh] rounded-lg border border-[var(--border-primary)] bg-[var(--bg-secondary)] shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-[var(--border-subtle)]">
          <h2 className="text-base font-semibold text-[var(--fg-primary)]">Settings</h2>
          <button
            onClick={toggleSettings}
            className="px-2 py-0.5 text-sm text-[var(--fg-muted)] hover:text-[var(--fg-primary)] hover:bg-[var(--hover-bg)] rounded"
            aria-label="Close settings"
          >
            Esc
          </button>
        </div>

        {/* Body */}
        <div className="flex-1 overflow-y-auto px-5 py-4 space-y-6">
          {/* Session Launch Section */}
          <Section title="Session Launch">
            <ToggleField
              ref={firstInputRef as React.Ref<HTMLInputElement>}
              label="Yolo Mode"
              description="Allow Copilot to run commands without confirmation"
              checked={config.yoloMode}
              onChange={(v) => updateField('yoloMode', v)}
              onReset={() => resetField('yoloMode')}
            />
            <TextField
              label="Agent"
              description="Agent name passed to Copilot CLI (leave empty for default)"
              value={config.agent}
              onChange={(v) => updateField('agent', v)}
              onReset={() => resetField('agent')}
              placeholder="e.g. coding-agent"
            />
            <TextField
              label="Model"
              description="Model name passed to Copilot CLI (leave empty for default)"
              value={config.model}
              onChange={(v) => updateField('model', v)}
              onReset={() => resetField('model')}
              placeholder="e.g. claude-sonnet-4"
            />
            <SelectField
              label="Launch Mode"
              description="How sessions open in the terminal"
              value={config.launch_mode}
              options={LAUNCH_MODES}
              onChange={(v) => updateField('launch_mode', v)}
              onReset={() => resetField('launch_mode')}
            />
            <SelectField
              label="Pane Direction"
              description="Split direction when launch mode is pane"
              value={config.pane_direction}
              options={PANE_DIRECTIONS}
              onChange={(v) => updateField('pane_direction', v)}
              onReset={() => resetField('pane_direction')}
            />
          </Section>

          {/* Terminal Section */}
          <Section title="Terminal">
            <TextField
              label="Terminal"
              description="Preferred terminal emulator"
              value={config.default_terminal}
              onChange={(v) => updateField('default_terminal', v)}
              onReset={() => resetField('default_terminal')}
              placeholder="e.g. Windows Terminal, iTerm2, Alacritty"
            />
            <TextField
              label="Shell"
              description="Preferred shell"
              value={config.default_shell}
              onChange={(v) => updateField('default_shell', v)}
              onReset={() => resetField('default_shell')}
              placeholder="e.g. pwsh, bash, zsh"
            />
            <TextField
              label="Custom Command"
              description="Custom command replacing default resume (use {sessionId} placeholder)"
              value={config.custom_command}
              onChange={(v) => updateField('custom_command', v)}
              onReset={() => resetField('custom_command')}
              placeholder="e.g. gh copilot resume {sessionId}"
            />
          </Section>

          {/* Appearance Section */}
          <Section title="Appearance">
            <SelectField
              label="Theme"
              description="Color scheme for the interface"
              value={themeValue}
              options={themeOptions}
              onChange={handleThemeChange}
              onReset={handleResetTheme}
            />
          </Section>

          {/* Advanced Section */}
          <Section title="Advanced">
            <ToggleField
              label="Workspace Recovery"
              description="Detect sessions interrupted by crash or reboot"
              checked={config.workspace_recovery}
              onChange={(v) => updateField('workspace_recovery', v)}
              onReset={() => resetField('workspace_recovery')}
            />
          </Section>
        </div>

        {/* Footer */}
        <div className="flex items-center justify-between px-5 py-3 border-t border-[var(--border-subtle)]">
          <button
            onClick={handleOpenConfigDir}
            className="text-xs text-[var(--fg-muted)] hover:text-[var(--accent-primary)] hover:underline truncate max-w-[280px]"
            title={configPath}
          >
            {configPath}
          </button>

          <div className="flex items-center gap-2">
            <button
              onClick={toggleSettings}
              className="px-3 py-1.5 text-sm text-[var(--fg-secondary)] hover:bg-[var(--hover-bg)] rounded border border-[var(--border-subtle)]"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="px-3 py-1.5 text-sm font-medium text-white bg-[var(--accent-primary)] hover:opacity-90 rounded disabled:opacity-50"
            >
              {isSaving ? 'Saving...' : 'Save'}
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Section                                                                     */
/* -------------------------------------------------------------------------- */

function Section({ title, children }: { title: string; children: React.ReactNode }) {
  return (
    <fieldset className="space-y-3">
      <legend className="text-xs font-semibold uppercase tracking-wider text-[var(--fg-muted)] mb-2">
        {title}
      </legend>
      {children}
    </fieldset>
  );
}

/* -------------------------------------------------------------------------- */
/* Toggle Field                                                                */
/* -------------------------------------------------------------------------- */

interface ToggleFieldProps {
  label: string;
  description: string;
  checked: boolean;
  onChange: (value: boolean) => void;
  onReset: () => void;
}

const ToggleField = React.forwardRef<HTMLInputElement, ToggleFieldProps>(
  function ToggleField({ label, description, checked, onChange, onReset }, ref) {
    return (
      <div className="flex items-center justify-between gap-4 py-1.5 group">
        <div className="flex-1 min-w-0">
          <div className="text-sm text-[var(--fg-primary)]">{label}</div>
          <div className="text-xs text-[var(--fg-muted)] mt-0.5">{description}</div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onReset}
            className="text-xs text-[var(--fg-muted)] hover:text-[var(--accent-primary)] opacity-0 group-hover:opacity-100 transition-opacity"
            title="Reset to default"
            tabIndex={-1}
          >
            ↺
          </button>
          <label className="relative inline-flex items-center cursor-pointer">
            <input
              ref={ref}
              type="checkbox"
              checked={checked}
              onChange={(e) => onChange(e.target.checked)}
              className="sr-only peer"
            />
            <div className="w-9 h-5 bg-[var(--bg-tertiary)] border border-[var(--border-primary)] peer-focus:ring-2 peer-focus:ring-[var(--focus-ring)] rounded-full peer peer-checked:bg-[var(--accent-primary)] after:content-[''] after:absolute after:top-[3px] after:left-[3px] after:bg-white after:rounded-full after:h-3.5 after:w-3.5 after:transition-transform peer-checked:after:translate-x-4" />
          </label>
        </div>
      </div>
    );
  }
);

/* -------------------------------------------------------------------------- */
/* Text Field                                                                  */
/* -------------------------------------------------------------------------- */

interface TextFieldProps {
  label: string;
  description: string;
  value: string;
  onChange: (value: string) => void;
  onReset: () => void;
  placeholder?: string;
}

function TextField({ label, description, value, onChange, onReset, placeholder }: TextFieldProps) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 group">
      <div className="flex-1 min-w-0">
        <div className="text-sm text-[var(--fg-primary)]">{label}</div>
        <div className="text-xs text-[var(--fg-muted)] mt-0.5">{description}</div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onReset}
          className="text-xs text-[var(--fg-muted)] hover:text-[var(--accent-primary)] opacity-0 group-hover:opacity-100 transition-opacity"
          title="Reset to default"
          tabIndex={-1}
        >
          ↺
        </button>
        <input
          type="text"
          value={value}
          onChange={(e) => onChange(e.target.value)}
          placeholder={placeholder}
          className="w-44 px-2 py-1 text-sm bg-[var(--bg-tertiary)] border border-[var(--border-primary)] rounded text-[var(--fg-primary)] placeholder-[var(--fg-muted)] focus:outline-none focus:ring-1 focus:ring-[var(--focus-ring)]"
        />
      </div>
    </div>
  );
}

/* -------------------------------------------------------------------------- */
/* Select Field                                                                */
/* -------------------------------------------------------------------------- */

interface SelectFieldProps {
  label: string;
  description: string;
  value: string;
  options: { value: string; label: string }[];
  onChange: (value: string) => void;
  onReset: () => void;
}

function SelectField({ label, description, value, options, onChange, onReset }: SelectFieldProps) {
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 group">
      <div className="flex-1 min-w-0">
        <div className="text-sm text-[var(--fg-primary)]">{label}</div>
        <div className="text-xs text-[var(--fg-muted)] mt-0.5">{description}</div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onReset}
          className="text-xs text-[var(--fg-muted)] hover:text-[var(--accent-primary)] opacity-0 group-hover:opacity-100 transition-opacity"
          title="Reset to default"
          tabIndex={-1}
        >
          ↺
        </button>
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-44 px-2 py-1 text-sm bg-[var(--bg-tertiary)] border border-[var(--border-primary)] rounded text-[var(--fg-primary)] focus:outline-none focus:ring-1 focus:ring-[var(--focus-ring)] appearance-none cursor-pointer"
        >
          {options.map((opt) => (
            <option key={opt.value} value={opt.value}>
              {opt.label}
            </option>
          ))}
        </select>
      </div>
    </div>
  );
}
