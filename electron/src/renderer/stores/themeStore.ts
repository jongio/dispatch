import { create } from 'zustand';
import { persist } from 'zustand/middleware';
import {
  type ThemeDefinition,
  themes,
  getThemeById,
  resolveAutoTheme,
} from '../styles/themes';

interface ThemeState {
  /** Theme name or 'auto' to follow system preference. */
  currentTheme: string;
  /** Actual theme id after resolving 'auto' against system preference. */
  resolvedTheme: string;
  /** All available theme definitions. */
  themes: ThemeDefinition[];
  /** Update the selected theme (pass a theme id or 'auto'). */
  setTheme: (name: string) => void;
  /** Called when system color-scheme preference changes (only relevant in 'auto' mode). */
  syncSystemPreference: (prefersDark: boolean) => void;
}

function resolve(current: string, prefersDark: boolean): string {
  if (current === 'auto') {
    return resolveAutoTheme(prefersDark).id;
  }
  return getThemeById(current).id;
}

/** Detect current system dark mode preference. */
function getSystemPrefersDark(): boolean {
  if (typeof window === 'undefined') return true;
  return window.matchMedia('(prefers-color-scheme: dark)').matches;
}

export const useThemeStore = create<ThemeState>()(
  persist(
    (set) => ({
      currentTheme: 'auto',
      resolvedTheme: resolve('auto', getSystemPrefersDark()),
      themes,

      setTheme: (name: string) => {
        const resolved = resolve(name, getSystemPrefersDark());
        set({ currentTheme: name, resolvedTheme: resolved });
      },

      syncSystemPreference: (prefersDark: boolean) => {
        set((state) => {
          if (state.currentTheme !== 'auto') return state;
          return { resolvedTheme: resolve('auto', prefersDark) };
        });
      },
    }),
    {
      name: 'dispatch-theme',
      // Only persist the user's selection, not the derived resolved theme.
      partialize: (state) => ({ currentTheme: state.currentTheme }),
      // After rehydration, re-resolve so the resolved theme is accurate.
      onRehydrateStorage: () => (state) => {
        if (!state) return;
        state.resolvedTheme = resolve(
          state.currentTheme,
          getSystemPrefersDark(),
        );
      },
    },
  ),
);
