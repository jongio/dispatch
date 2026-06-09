import { useEffect } from 'react';
import { useThemeStore } from '../stores/themeStore';
import { getThemeById } from '../styles/themes';
import type { ThemeColors, ThemeDefinition } from '../styles/themes';

/**
 * Apply the active theme's CSS variables to `document.documentElement` and
 * sync the `data-theme` attribute (for any CSS selectors that rely on it).
 *
 * Also listens to the system `prefers-color-scheme` media query so 'auto'
 * mode tracks OS appearance changes in real time.
 *
 * Usage: call `useTheme()` once at the app root (e.g. in `<App />`).
 */
export function useTheme(): {
  currentTheme: string;
  resolvedTheme: string;
  themes: ThemeDefinition[];
  setTheme: (name: string) => void;
} {
  const { currentTheme, resolvedTheme, themes, setTheme, syncSystemPreference } =
    useThemeStore();

  // Apply CSS variables whenever the resolved theme changes.
  useEffect(() => {
    const theme = getThemeById(resolvedTheme);
    const root = document.documentElement;

    // Set each CSS custom property.
    const entries = Object.entries(theme.colors) as [keyof ThemeColors, string][];
    for (const [prop, value] of entries) {
      root.style.setProperty(`--${prop}`, value);
    }

    // Set data-theme attribute for any CSS rules that branch on it.
    root.setAttribute('data-theme', theme.appearance);
    // Set color-scheme so native controls (scrollbars, inputs) match.
    root.style.setProperty('color-scheme', theme.appearance);
  }, [resolvedTheme]);

  // Listen to OS color-scheme changes for 'auto' mode.
  useEffect(() => {
    const mq = window.matchMedia('(prefers-color-scheme: dark)');

    const handler = (e: MediaQueryListEvent) => {
      syncSystemPreference(e.matches);
    };

    mq.addEventListener('change', handler);
    return () => mq.removeEventListener('change', handler);
  }, [syncSystemPreference]);

  return { currentTheme, resolvedTheme, themes, setTheme };
}
