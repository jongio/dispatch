// Apply persisted theme before first paint to prevent flash of wrong colors.
// This runs as an inline script in the HTML head, before React mounts.
import { loadAndApplyTheme } from './lib/applyTheme';

loadAndApplyTheme();
