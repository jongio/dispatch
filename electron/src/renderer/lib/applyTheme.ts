/**
 * Theme application and system preference detection.
 *
 * Applies a ThemeDefinition by setting CSS custom properties on :root and
 * persisting the selection to localStorage.
 */

import type { ThemeColors, ThemeDefinition } from './themes';
import { BUILTIN_THEMES, DEFAULT_THEME, getThemeById } from './themes';

const STORAGE_KEY = 'dispatch-theme';

/**
 * Convert a camelCase color key to its CSS custom property name.
 * e.g. "cardForeground" => "--card-foreground"
 */
function colorKeyToCssVar(key: string): string {
  return '--' + key.replace(/([A-Z])/g, '-$1').toLowerCase();
}

/**
 * Apply a theme by setting all CSS custom properties on the document root.
 * Also sets the data-theme attribute (for any CSS selectors that rely on it)
 * and persists the theme ID to localStorage.
 */
export function applyTheme(theme: ThemeDefinition): void {
  const root = document.documentElement;

  // Set CSS custom properties
  const entries = Object.entries(theme.colors) as [keyof ThemeColors, string][];
  for (const [key, value] of entries) {
    root.style.setProperty(colorKeyToCssVar(key), value);
  }

  // Set data-theme for any residual CSS selectors and class-based dark mode
  root.setAttribute('data-theme', theme.id);

  // Set the class for Tailwind dark mode detection if needed
  if (theme.isDark) {
    root.classList.add('dark');
  } else {
    root.classList.remove('dark');
  }

  // Persist
  localStorage.setItem(STORAGE_KEY, theme.id);
}

/**
 * Load and apply the persisted theme. Falls back to DEFAULT_THEME
 * if no valid theme is stored.
 */
export function loadAndApplyTheme(): ThemeDefinition {
  const storedId = localStorage.getItem(STORAGE_KEY);
  const theme = (storedId && getThemeById(storedId)) || DEFAULT_THEME;
  applyTheme(theme);
  return theme;
}

/**
 * Get the currently active theme ID from localStorage.
 */
export function getActiveThemeId(): string {
  return localStorage.getItem(STORAGE_KEY) || DEFAULT_THEME.id;
}

/**
 * Find the best matching built-in theme for a Windows Terminal scheme name.
 * Returns undefined if no match.
 */
export function matchWindowsTerminalScheme(schemeName: string): ThemeDefinition | undefined {
  const lower = schemeName.toLowerCase();

  const mapping: Record<string, string> = {
    'campbell': 'campbell',
    'one half dark': 'one-half-dark',
    'one half light': 'one-half-light',
  };

  const themeId = mapping[lower];
  return themeId ? getThemeById(themeId) : undefined;
}

/**
 * Detect system preference for high contrast and apply overrides.
 * Returns a cleanup function to remove the listener.
 */
export function watchHighContrast(): () => void {
  const mq = window.matchMedia('(prefers-contrast: more)');

  function applyHighContrast(matches: boolean) {
    const root = document.documentElement;
    if (matches) {
      root.style.setProperty('--border', '#666666');
      root.style.setProperty('--muted-foreground', '#CCCCCC');
    } else {
      // Re-apply current theme values
      const storedId = localStorage.getItem(STORAGE_KEY);
      const theme = (storedId && getThemeById(storedId)) || DEFAULT_THEME;
      root.style.setProperty('--border', theme.colors.border);
      root.style.setProperty('--muted-foreground', theme.colors.mutedForeground);
    }
  }

  applyHighContrast(mq.matches);

  const handler = (e: MediaQueryListEvent) => applyHighContrast(e.matches);
  mq.addEventListener('change', handler);
  return () => mq.removeEventListener('change', handler);
}

/**
 * Detect system color scheme preference (dark/light) and pick the
 * closest Dispatch theme. Used for the "auto" option.
 */
export function getSystemPreferredTheme(): ThemeDefinition {
  const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches;
  return prefersDark ? BUILTIN_THEMES[0] : BUILTIN_THEMES[1];
}
