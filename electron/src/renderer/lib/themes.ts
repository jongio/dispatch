/**
 * Built-in theme definitions for Dispatch.
 *
 * Each theme provides a complete set of CSS color values (hex) that map to
 * the application's design token system. The five themes mirror the Go TUI
 * built-in color schemes.
 */

export interface ThemeColors {
  background: string;
  foreground: string;
  card: string;
  cardForeground: string;
  muted: string;
  mutedForeground: string;
  border: string;
  input: string;
  ring: string;
  primary: string;
  primaryForeground: string;
  secondary: string;
  secondaryForeground: string;
  accent: string;
  accentForeground: string;
  destructive: string;
  destructiveForeground: string;
}

export interface ThemeDefinition {
  id: string;
  name: string;
  isDark: boolean;
  colors: ThemeColors;
}

/**
 * Derives card, muted, border, and secondary tones from a base background
 * by shifting lightness. Dark themes lighten; light themes darken.
 */
function deriveDarkPalette(bg: string, primary: string): Pick<
  ThemeColors,
  'card' | 'cardForeground' | 'muted' | 'mutedForeground' | 'border' | 'input' | 'secondary' | 'secondaryForeground'
> {
  // For dark themes we use pre-computed values relative to the bg tone
  void bg;
  void primary;
  return {
    card: lightenHex(bg, 0.04),
    cardForeground: '#E4E4E7',
    muted: lightenHex(bg, 0.08),
    mutedForeground: '#A1A1AA',
    border: lightenHex(bg, 0.12),
    input: lightenHex(bg, 0.12),
    secondary: lightenHex(bg, 0.10),
    secondaryForeground: '#E4E4E7',
  };
}

function deriveLightPalette(bg: string, primary: string): Pick<
  ThemeColors,
  'card' | 'cardForeground' | 'muted' | 'mutedForeground' | 'border' | 'input' | 'secondary' | 'secondaryForeground'
> {
  void bg;
  void primary;
  return {
    card: '#FFFFFF',
    cardForeground: '#1A1A2E',
    muted: darkenHex(bg, 0.04),
    mutedForeground: '#6B7280',
    border: darkenHex(bg, 0.10),
    input: darkenHex(bg, 0.10),
    secondary: darkenHex(bg, 0.06),
    secondaryForeground: '#1A1A2E',
  };
}

/** Lighten a hex color by mixing toward white. Amount is 0..1. */
function lightenHex(hex: string, amount: number): string {
  const [r, g, b] = parseHex(hex);
  return toHex(
    Math.round(r + (255 - r) * amount),
    Math.round(g + (255 - g) * amount),
    Math.round(b + (255 - b) * amount),
  );
}

/** Darken a hex color by mixing toward black. Amount is 0..1. */
function darkenHex(hex: string, amount: number): string {
  const [r, g, b] = parseHex(hex);
  return toHex(
    Math.round(r * (1 - amount)),
    Math.round(g * (1 - amount)),
    Math.round(b * (1 - amount)),
  );
}

function parseHex(hex: string): [number, number, number] {
  const h = hex.replace('#', '');
  return [
    parseInt(h.slice(0, 2), 16),
    parseInt(h.slice(2, 4), 16),
    parseInt(h.slice(4, 6), 16),
  ];
}

function toHex(r: number, g: number, b: number): string {
  const clamp = (v: number) => Math.max(0, Math.min(255, v));
  return `#${clamp(r).toString(16).padStart(2, '0')}${clamp(g).toString(16).padStart(2, '0')}${clamp(b).toString(16).padStart(2, '0')}`;
}

/* -------------------------------------------------------------------------- */
/* Theme Definitions                                                           */
/* -------------------------------------------------------------------------- */

const DISPATCH_DARK: ThemeDefinition = {
  id: 'dispatch-dark',
  name: 'Dispatch Dark',
  isDark: true,
  colors: {
    background: '#111111',
    foreground: '#E4E4E7',
    ...deriveDarkPalette('#111111', '#5A56E0'),
    ring: '#5A56E0',
    primary: '#5A56E0',
    primaryForeground: '#FFFFFF',
    accent: '#3A96DD',
    accentForeground: '#FFFFFF',
    destructive: '#E54D2E',
    destructiveForeground: '#FFFFFF',
  },
};

const DISPATCH_LIGHT: ThemeDefinition = {
  id: 'dispatch-light',
  name: 'Dispatch Light',
  isDark: false,
  colors: {
    background: '#FAFAFA',
    foreground: '#1A1A2E',
    ...deriveLightPalette('#FAFAFA', '#5A56E0'),
    ring: '#5A56E0',
    primary: '#5A56E0',
    primaryForeground: '#FFFFFF',
    accent: '#3A96DD',
    accentForeground: '#FFFFFF',
    destructive: '#DC2626',
    destructiveForeground: '#FFFFFF',
  },
};

const CAMPBELL: ThemeDefinition = {
  id: 'campbell',
  name: 'Campbell',
  isDark: true,
  colors: {
    background: '#0C0C0C',
    foreground: '#CCCCCC',
    ...deriveDarkPalette('#0C0C0C', '#0037DA'),
    ring: '#0037DA',
    primary: '#0037DA',
    primaryForeground: '#FFFFFF',
    accent: '#3A96DD',
    accentForeground: '#FFFFFF',
    destructive: '#C50F1F',
    destructiveForeground: '#FFFFFF',
  },
};

const ONE_HALF_DARK: ThemeDefinition = {
  id: 'one-half-dark',
  name: 'One Half Dark',
  isDark: true,
  colors: {
    background: '#282C34',
    foreground: '#DCDFE4',
    ...deriveDarkPalette('#282C34', '#61AFEF'),
    ring: '#61AFEF',
    primary: '#61AFEF',
    primaryForeground: '#282C34',
    accent: '#56B6C2',
    accentForeground: '#282C34',
    destructive: '#E06C75',
    destructiveForeground: '#282C34',
  },
};

const ONE_HALF_LIGHT: ThemeDefinition = {
  id: 'one-half-light',
  name: 'One Half Light',
  isDark: false,
  colors: {
    background: '#FAFAFA',
    foreground: '#383A42',
    ...deriveLightPalette('#FAFAFA', '#0184BC'),
    ring: '#0184BC',
    primary: '#0184BC',
    primaryForeground: '#FFFFFF',
    accent: '#0997B3',
    accentForeground: '#FFFFFF',
    destructive: '#E45649',
    destructiveForeground: '#FFFFFF',
  },
};

/* -------------------------------------------------------------------------- */
/* Exports                                                                     */
/* -------------------------------------------------------------------------- */

export const BUILTIN_THEMES: ThemeDefinition[] = [
  DISPATCH_DARK,
  DISPATCH_LIGHT,
  CAMPBELL,
  ONE_HALF_DARK,
  ONE_HALF_LIGHT,
];

export const THEME_NAMES: string[] = BUILTIN_THEMES.map((t) => t.name);

export const DEFAULT_THEME = DISPATCH_DARK;

export function getThemeById(id: string): ThemeDefinition | undefined {
  return BUILTIN_THEMES.find((t) => t.id === id);
}

export function getThemeByName(name: string): ThemeDefinition | undefined {
  return BUILTIN_THEMES.find((t) => t.name === name);
}
