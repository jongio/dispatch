/**
 * Theme definitions for Dispatch.
 *
 * Each theme maps CSS custom property names (without the `--` prefix)
 * to hex color values. The properties are applied to `document.documentElement`
 * so that all `var(--*)` references resolve at runtime.
 *
 * Contrast targets:
 *   fg-primary on bg-primary  >= 7:1
 *   fg-secondary on bg-primary >= 4.5:1
 *   fg-muted on bg-primary    >= 3:1 (decorative/non-essential)
 *   accent-primary on bg-primary >= 4.5:1
 */

export interface ThemeColors {
  'bg-primary': string;
  'bg-secondary': string;
  'bg-tertiary': string;
  'fg-primary': string;
  'fg-secondary': string;
  'fg-muted': string;
  'accent-primary': string;
  'accent-secondary': string;
  'border-primary': string;
  'border-subtle': string;
  'hover-bg': string;
  'selection-bg': string;
  'focus-ring': string;
  'attention-working': string;
  'attention-thinking': string;
  'attention-compacting': string;
  'attention-waiting': string;
  'attention-active': string;
  'attention-stale': string;
  'attention-interrupted': string;
  'attention-idle': string;
  'conversation-user-bg': string;
  'conversation-assistant-bg': string;
}

export interface ThemeDefinition {
  /** Unique key used in storage and selectors. */
  id: string;
  /** Human-readable display name. */
  name: string;
  /** Whether this is a dark or light appearance. */
  appearance: 'dark' | 'light';
  /** CSS custom property values. */
  colors: ThemeColors;
}

// ---------------------------------------------------------------------------
// Theme: Dispatch Dark (default — Tokyo Night-inspired)
// ---------------------------------------------------------------------------
export const dispatchDark: ThemeDefinition = {
  id: 'dispatch-dark',
  name: 'Dispatch Dark',
  appearance: 'dark',
  colors: {
    'bg-primary': '#1a1b26',
    'bg-secondary': '#16161e',
    'bg-tertiary': '#24283b',
    'fg-primary': '#c0caf5',
    'fg-secondary': '#a9b1d6',
    'fg-muted': '#565f89',
    'accent-primary': '#7aa2f7',
    'accent-secondary': '#bb9af7',
    'border-primary': '#3b4261',
    'border-subtle': '#292e42',
    'hover-bg': '#292e42',
    'selection-bg': '#283457',
    'focus-ring': '#7aa2f7',
    'attention-working': '#7aa2f7',
    'attention-thinking': '#7dcfff',
    'attention-compacting': '#bb9af7',
    'attention-waiting': '#9d7cd8',
    'attention-active': '#9ece6a',
    'attention-stale': '#e0af68',
    'attention-interrupted': '#ff9e64',
    'attention-idle': '#565f89',
    'conversation-user-bg': '#283457',
    'conversation-assistant-bg': '#1f2335',
  },
};

// ---------------------------------------------------------------------------
// Theme: Dispatch Light
// ---------------------------------------------------------------------------
export const dispatchLight: ThemeDefinition = {
  id: 'dispatch-light',
  name: 'Dispatch Light',
  appearance: 'light',
  colors: {
    'bg-primary': '#f5f5f5',
    'bg-secondary': '#ffffff',
    'bg-tertiary': '#e8e8e8',
    'fg-primary': '#1a1b26',
    'fg-secondary': '#4c566a',
    'fg-muted': '#9aa5ce',
    'accent-primary': '#2e7de9',
    'accent-secondary': '#7847bd',
    'border-primary': '#d4d4d8',
    'border-subtle': '#e4e4e7',
    'hover-bg': '#e8e8eb',
    'selection-bg': '#dbeafe',
    'focus-ring': '#2e7de9',
    'attention-working': '#2e7de9',
    'attention-thinking': '#007197',
    'attention-compacting': '#7847bd',
    'attention-waiting': '#6f42c1',
    'attention-active': '#2e7d32',
    'attention-stale': '#e65100',
    'attention-interrupted': '#d32f2f',
    'attention-idle': '#9aa5ce',
    'conversation-user-bg': '#dbeafe',
    'conversation-assistant-bg': '#f0f0f0',
  },
};

// ---------------------------------------------------------------------------
// Theme: Campbell (Windows Terminal default dark)
// ---------------------------------------------------------------------------
export const campbell: ThemeDefinition = {
  id: 'campbell',
  name: 'Campbell',
  appearance: 'dark',
  colors: {
    'bg-primary': '#0c0c0c',
    'bg-secondary': '#1e1e1e',
    'bg-tertiary': '#2d2d2d',
    'fg-primary': '#cccccc',
    'fg-secondary': '#999999',
    'fg-muted': '#767676',
    'accent-primary': '#3b78ff',
    'accent-secondary': '#b4009e',
    'border-primary': '#404040',
    'border-subtle': '#2d2d2d',
    'hover-bg': '#2d2d2d',
    'selection-bg': '#264f78',
    'focus-ring': '#3b78ff',
    'attention-working': '#3b78ff',
    'attention-thinking': '#3a96dd',
    'attention-compacting': '#881798',
    'attention-waiting': '#b4009e',
    'attention-active': '#16c60c',
    'attention-stale': '#c19c00',
    'attention-interrupted': '#e74856',
    'attention-idle': '#767676',
    'conversation-user-bg': '#264f78',
    'conversation-assistant-bg': '#1e1e1e',
  },
};

// ---------------------------------------------------------------------------
// Theme: One Half Dark
// ---------------------------------------------------------------------------
export const oneHalfDark: ThemeDefinition = {
  id: 'one-half-dark',
  name: 'One Half Dark',
  appearance: 'dark',
  colors: {
    'bg-primary': '#282c34',
    'bg-secondary': '#21252b',
    'bg-tertiary': '#2c313a',
    'fg-primary': '#abb2bf',
    'fg-secondary': '#828997',
    'fg-muted': '#5c6370',
    'accent-primary': '#61afef',
    'accent-secondary': '#c678dd',
    'border-primary': '#3e4452',
    'border-subtle': '#2c313a',
    'hover-bg': '#2c313a',
    'selection-bg': '#3e4452',
    'focus-ring': '#61afef',
    'attention-working': '#61afef',
    'attention-thinking': '#56b6c2',
    'attention-compacting': '#c678dd',
    'attention-waiting': '#c678dd',
    'attention-active': '#98c379',
    'attention-stale': '#e5c07b',
    'attention-interrupted': '#e06c75',
    'attention-idle': '#5c6370',
    'conversation-user-bg': '#2b3d4f',
    'conversation-assistant-bg': '#21252b',
  },
};

// ---------------------------------------------------------------------------
// Theme: One Half Light
// ---------------------------------------------------------------------------
export const oneHalfLight: ThemeDefinition = {
  id: 'one-half-light',
  name: 'One Half Light',
  appearance: 'light',
  colors: {
    'bg-primary': '#fafafa',
    'bg-secondary': '#ffffff',
    'bg-tertiary': '#eaeaeb',
    'fg-primary': '#383a42',
    'fg-secondary': '#696c77',
    'fg-muted': '#a0a1a7',
    'accent-primary': '#0184bc',
    'accent-secondary': '#a626a4',
    'border-primary': '#d4d4d8',
    'border-subtle': '#e5e5e6',
    'hover-bg': '#eaeaeb',
    'selection-bg': '#bfdbfe',
    'focus-ring': '#0184bc',
    'attention-working': '#0184bc',
    'attention-thinking': '#0997b3',
    'attention-compacting': '#a626a4',
    'attention-waiting': '#a626a4',
    'attention-active': '#50a14f',
    'attention-stale': '#c18401',
    'attention-interrupted': '#e45649',
    'attention-idle': '#a0a1a7',
    'conversation-user-bg': '#dbeafe',
    'conversation-assistant-bg': '#f0f0f0',
  },
};

// ---------------------------------------------------------------------------
// Registry — ordered list of all available themes
// ---------------------------------------------------------------------------
export const themes: ThemeDefinition[] = [
  dispatchDark,
  dispatchLight,
  campbell,
  oneHalfDark,
  oneHalfLight,
];

/** Look up a theme by id. Returns `dispatchDark` as fallback. */
export function getThemeById(id: string): ThemeDefinition {
  return themes.find((t) => t.id === id) ?? dispatchDark;
}

/** Resolve 'auto' to a concrete theme based on system appearance. */
export function resolveAutoTheme(prefersDark: boolean): ThemeDefinition {
  return prefersDark ? dispatchDark : dispatchLight;
}
