import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSessionStore } from '../stores/sessionStore';
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

const THEME_OPTIONS = [
  { value: 'auto', label: 'Auto (System)' },
  { value: 'dark', label: 'Dark' },
  { value: 'light', label: 'Light' },
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
  const [config, setConfig] = useState<Config>(getDefaultConfig());
  const [configPath, setConfigPath] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [shells, setShells] = useState<{ value: string; label: string }[]>([]);
  const [terminals, setTerminals] = useState<{ value: string; label: string }[]>([]);
  const [currentTheme, setCurrentTheme] = useState<string>(() => {
    return document.documentElement.getAttribute('data-theme') || 'dark';
  });
  const overlayRef = useRef<HTMLDivElement>(null);
  const firstInputRef = useRef<HTMLInputElement>(null);

  // Load config + detected shells/terminals when modal opens
  useEffect(() => {
    if (!showSettings) return;

    async function loadConfig() {
      const [cfg, path, detectedShells, detectedTerminals] = await Promise.all([
        window.dispatch.config.get(),
        window.dispatch.config.getPath(),
        window.dispatch.platform.getShells(),
        window.dispatch.platform.getTerminals(),
      ]);
      setConfig(cfg);
      setConfigPath(path);
      setShells([
        { value: '', label: 'Auto-detect' },
        ...detectedShells.map((s) => ({ value: s.name, label: s.displayName })),
      ]);
      setTerminals([
        { value: '', label: 'Auto-detect' },
        ...detectedTerminals.map((t) => ({ value: t.name, label: t.displayName })),
      ]);
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
    const themeId = value === 'auto' ? 'dark' : value;
    document.documentElement.setAttribute('data-theme', themeId);
    setCurrentTheme(value);
    setConfig((prev) => ({ ...prev, theme: value === 'auto' ? '' : value }));
  }, []);

  const handleResetTheme = useCallback(() => {
    document.documentElement.setAttribute('data-theme', 'dark');
    setCurrentTheme('auto');
    setConfig((prev) => ({ ...prev, theme: '' }));
  }, []);

  const handleOpenConfigDir = useCallback(() => {
    window.dispatch.config.openInExplorer();
  }, []);

  if (!showSettings) return null;

  const themeValue = currentTheme;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={handleBackdropClick}
      onKeyDown={handleKeyDown}
    >
      <div
        className="flex flex-col w-[600px] max-h-[85vh] rounded-lg border border-border bg-card shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-label="Settings"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-border">
          <h2 className="text-base font-semibold text-foreground">Settings</h2>
          <button
            onClick={toggleSettings}
            className="px-2 py-0.5 text-sm text-muted-foreground hover:text-foreground hover:bg-muted/30 rounded"
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
            <SelectField
              label="Terminal"
              description="Preferred terminal emulator"
              value={config.default_terminal}
              options={terminals}
              onChange={(v) => updateField('default_terminal', v)}
              onReset={() => resetField('default_terminal')}
            />
            <SelectField
              label="Shell"
              description="Preferred shell"
              value={config.default_shell}
              options={shells}
              onChange={(v) => updateField('default_shell', v)}
              onReset={() => resetField('default_shell')}
            />
            <div className="space-y-1">
              <ToggleField
                label="Use Custom Command"
                description="Override the default gh copilot session resume command"
                checked={config.custom_command !== ''}
                onChange={(v) => {
                  if (!v) updateField('custom_command', '');
                }}
                onReset={() => resetField('custom_command')}
              />
              <TextField
                label="Custom Command"
                description="Command to run (use {sessionId} as placeholder for the session ID)"
                value={config.custom_command}
                onChange={(v) => updateField('custom_command', v)}
                onReset={() => resetField('custom_command')}
                placeholder="e.g. gh copilot resume {sessionId}"
                disabled={config.custom_command === '' && false}
              />
            </div>
          </Section>

          {/* Appearance Section */}
          <Section title="Appearance">
            <SelectField
              label="Theme"
              description="Color scheme for the interface"
              value={themeValue}
              options={THEME_OPTIONS}
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
        <div className="flex items-center justify-between px-5 py-3 border-t border-border">
          <button
            onClick={handleOpenConfigDir}
            className="text-xs text-muted-foreground hover:text-primary hover:underline truncate max-w-[280px]"
            title={configPath}
          >
            {configPath}
          </button>

          <div className="flex items-center gap-2">
            <button
              onClick={toggleSettings}
              className="px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted/30 rounded border border-border"
            >
              Cancel
            </button>
            <button
              onClick={handleSave}
              disabled={isSaving}
              className="px-3 py-1.5 text-sm font-medium text-primary-foreground bg-primary hover:opacity-90 rounded disabled:opacity-50"
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
      <legend className="text-xs font-semibold uppercase tracking-wider text-muted-foreground mb-2">
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
          <div className="text-sm text-foreground">{label}</div>
          <div className="text-xs text-muted-foreground mt-0.5">{description}</div>
        </div>
        <div className="flex items-center gap-2">
          <button
            onClick={onReset}
            className="text-xs text-muted-foreground hover:text-primary opacity-0 group-hover:opacity-100 transition-opacity"
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
            <div className="w-9 h-5 bg-muted border border-border peer-focus:ring-2 peer-focus:ring-ring rounded-full peer peer-checked:bg-primary after:content-[''] after:absolute after:top-[3px] after:left-[3px] after:bg-white after:rounded-full after:h-3.5 after:w-3.5 after:transition-transform peer-checked:after:translate-x-4" />
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
  disabled?: boolean;
}

function TextField({ label, description, value, onChange, onReset, placeholder, disabled }: TextFieldProps) {
  return (
    <div className={`flex items-center justify-between gap-4 py-1.5 group ${disabled ? 'opacity-50' : ''}`}>
      <div className="flex-1 min-w-0">
        <div className="text-sm text-foreground">{label}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{description}</div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onReset}
          className="text-xs text-muted-foreground hover:text-primary opacity-0 group-hover:opacity-100 transition-opacity"
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
          disabled={disabled}
          className="w-44 px-2 py-1 text-sm bg-muted border border-border rounded text-foreground placeholder:text-muted-foreground focus:outline-none focus:ring-1 focus:ring-ring disabled:cursor-not-allowed"
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
        <div className="text-sm text-foreground">{label}</div>
        <div className="text-xs text-muted-foreground mt-0.5">{description}</div>
      </div>
      <div className="flex items-center gap-2">
        <button
          onClick={onReset}
          className="text-xs text-muted-foreground hover:text-primary opacity-0 group-hover:opacity-100 transition-opacity"
          title="Reset to default"
          tabIndex={-1}
        >
          ↺
        </button>
        <select
          value={value}
          onChange={(e) => onChange(e.target.value)}
          className="w-44 px-2 py-1 text-sm bg-muted border border-border rounded text-foreground focus:outline-none focus:ring-1 focus:ring-ring appearance-none cursor-pointer"
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
