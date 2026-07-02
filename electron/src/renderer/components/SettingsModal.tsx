import React, { useCallback, useEffect, useRef, useState } from 'react';
import { useSessionStore } from '../stores/sessionStore';
import type { Config } from '../../preload/index';
import { BUILTIN_THEMES, type ThemeDefinition } from '../lib/themes';
import { applyTheme, getActiveThemeId } from '../lib/applyTheme';
import { useFocusTrap } from '../lib/useFocusTrap';

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
    custom_command: '',
    theme: '',
    workspace_recovery: true,
    global_hotkey: 'CommandOrControl+Shift+D',
    auto_launch: false,
    auto_update: true,
    notifications_enabled: true,
    minimize_to_tray: true,
  };
}

export function SettingsModal() {
  const { showSettings, toggleSettings } = useSessionStore();
  const [config, setConfig] = useState<Config>(getDefaultConfig());
  const [configPath, setConfigPath] = useState('');
  const [isSaving, setIsSaving] = useState(false);
  const [shells, setShells] = useState<{ value: string; label: string }[]>([]);
  const [terminals, setTerminals] = useState<{ value: string; label: string }[]>([]);
  const [activeThemeId, setActiveThemeId] = useState<string>(getActiveThemeId);
  const overlayRef = useRef<HTMLDivElement>(null);
  const firstInputRef = useRef<HTMLInputElement>(null);
  const trapRef = useFocusTrap(showSettings);
  const previousFocusRef = useRef<HTMLElement | null>(null);

  // Store the element that had focus before modal opened and restore on close
  useEffect(() => {
    if (showSettings) {
      previousFocusRef.current = document.activeElement as HTMLElement;
    } else if (previousFocusRef.current) {
      previousFocusRef.current.focus();
      previousFocusRef.current = null;
    }
  }, [showSettings]);

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
      const configToSave = { ...config, theme: activeThemeId };
      await window.dispatch.config.set(configToSave);
      toggleSettings();
    } finally {
      setIsSaving(false);
    }
  }, [config, activeThemeId, toggleSettings]);

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

  const handleThemeSelect = useCallback((theme: ThemeDefinition) => {
    applyTheme(theme);
    setActiveThemeId(theme.id);
    setConfig((prev) => ({ ...prev, theme: theme.id }));
  }, []);

  const handleOpenConfigDir = useCallback(() => {
    window.dispatch.config.openInExplorer();
  }, []);

  if (!showSettings) return null;

  return (
    <div
      ref={overlayRef}
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-sm"
      onClick={handleBackdropClick}
      onKeyDown={handleKeyDown}
    >
      <div
        ref={trapRef}
        className="flex flex-col w-[600px] max-h-[85vh] rounded-lg border border-border bg-card shadow-2xl"
        role="dialog"
        aria-modal="true"
        aria-labelledby="settings-modal-title"
      >
        {/* Header */}
        <div className="flex items-center justify-between px-5 py-3 border-b border-border">
          <h2 id="settings-modal-title" className="text-base font-semibold text-foreground">Settings</h2>
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
            <ThemePicker
              themes={BUILTIN_THEMES}
              activeThemeId={activeThemeId}
              onSelect={handleThemeSelect}
            />
          </Section>

          {/* Advanced Section */}
          <Section title="Advanced">
            <ToggleField
              label="Auto-launch on Startup"
              description="Start Dispatch automatically when you log in"
              checked={config.auto_launch}
              onChange={(v) => updateField('auto_launch', v)}
              onReset={() => resetField('auto_launch')}
            />
            <ToggleField
              label="Auto-update"
              description="Check for and install updates automatically"
              checked={config.auto_update}
              onChange={(v) => updateField('auto_update', v)}
              onReset={() => resetField('auto_update')}
            />
            <ToggleField
              label="Show Notifications"
              description="Native OS notifications when sessions need attention"
              checked={config.notifications_enabled}
              onChange={(v) => updateField('notifications_enabled', v)}
              onReset={() => resetField('notifications_enabled')}
            />
            <ToggleField
              label="Minimize to Tray"
              description="Hide to system tray instead of closing the window"
              checked={config.minimize_to_tray}
              onChange={(v) => updateField('minimize_to_tray', v)}
              onReset={() => resetField('minimize_to_tray')}
            />
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
    const descId = `toggle-desc-${label.replace(/\s+/g, '-').toLowerCase()}`;
    return (
      <div className="flex items-center justify-between gap-4 py-1.5 group">
        <div className="flex-1 min-w-0">
          <div className="text-sm text-foreground">{label}</div>
          <div id={descId} className="text-xs text-muted-foreground mt-0.5">{description}</div>
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
              aria-label={label}
              aria-describedby={descId}
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
  const descId = `text-desc-${label.replace(/\s+/g, '-').toLowerCase()}`;
  return (
    <div className={`flex items-center justify-between gap-4 py-1.5 group ${disabled ? 'opacity-50' : ''}`}>
      <div className="flex-1 min-w-0">
        <div className="text-sm text-foreground">{label}</div>
        <div id={descId} className="text-xs text-muted-foreground mt-0.5">{description}</div>
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
          aria-label={label}
          aria-describedby={descId}
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
  const descId = `select-desc-${label.replace(/\s+/g, '-').toLowerCase()}`;
  return (
    <div className="flex items-center justify-between gap-4 py-1.5 group">
      <div className="flex-1 min-w-0">
        <div className="text-sm text-foreground">{label}</div>
        <div id={descId} className="text-xs text-muted-foreground mt-0.5">{description}</div>
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
          aria-label={label}
          aria-describedby={descId}
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

/* -------------------------------------------------------------------------- */
/* Theme Picker                                                                */
/* -------------------------------------------------------------------------- */

interface ThemePickerProps {
  themes: ThemeDefinition[];
  activeThemeId: string;
  onSelect: (theme: ThemeDefinition) => void;
}

function ThemePicker({ themes, activeThemeId, onSelect }: ThemePickerProps) {
  return (
    <div className="space-y-2">
      <div className="text-sm text-foreground">Theme</div>
      <div className="text-xs text-muted-foreground">Choose a color scheme for the interface</div>
      <div className="grid grid-cols-1 gap-2 pt-1">
        {themes.map((theme) => {
          const isActive = theme.id === activeThemeId;
          return (
            <button
              key={theme.id}
              onClick={() => onSelect(theme)}
              className={`flex items-center gap-3 px-3 py-2 rounded border text-left transition-colors ${
                isActive
                  ? 'border-primary bg-primary/10 ring-1 ring-primary'
                  : 'border-border hover:border-muted-foreground hover:bg-muted/30'
              }`}
              aria-pressed={isActive}
            >
              {/* Color preview swatches */}
              <div className="flex gap-1 shrink-0">
                <span
                  className="w-4 h-4 rounded-full border border-black/20"
                  style={{ backgroundColor: theme.colors.background }}
                  title="Background"
                />
                <span
                  className="w-4 h-4 rounded-full border border-black/20"
                  style={{ backgroundColor: theme.colors.primary }}
                  title="Primary"
                />
                <span
                  className="w-4 h-4 rounded-full border border-black/20"
                  style={{ backgroundColor: theme.colors.accent }}
                  title="Accent"
                />
                <span
                  className="w-4 h-4 rounded-full border border-black/20"
                  style={{ backgroundColor: theme.colors.foreground }}
                  title="Foreground"
                />
              </div>
              {/* Theme name */}
              <span className="text-sm text-foreground">{theme.name}</span>
              {/* Dark/Light badge */}
              <span className="ml-auto text-[10px] uppercase tracking-wider text-muted-foreground">
                {theme.isDark ? 'dark' : 'light'}
              </span>
            </button>
          );
        })}
      </div>
    </div>
  );
}

