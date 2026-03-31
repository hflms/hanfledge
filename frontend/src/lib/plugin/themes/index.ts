// -- Theme System -----------------------------------------------

export interface ThemeColors {
  bgPrimary: string;
  bgSecondary: string;
  bgTertiary: string;
  bgCard: string;
  bgCardHover: string;
  bgGlass: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  accent: string;
  accentLight: string;
  accentGlow: string;
  border: string;
  borderFocus: string;
  success: string;
  warning: string;
  danger: string;
  info: string;
  shadowSm: string;
  shadowMd: string;
  shadowLg: string;
}

export interface ThemeConfig {
  id: string;
  name: string;
  description: string;
  type: 'dark' | 'light' | 'high-contrast';
  colors: ThemeColors;
  // School branding overrides
  schoolLogo?: string;
  schoolName?: string;
  customCSS?: string;
}

/**
 * Maps ThemeColors keys to CSS custom property names.
 * These MUST match the variable names used in globals.css
 * so that theme switching affects ALL components.
 */
export const THEME_CSS_VAR_MAP: Record<keyof ThemeColors, string> = {
  bgPrimary: '--bg-primary',
  bgSecondary: '--bg-secondary',
  bgTertiary: '--bg-tertiary',
  bgCard: '--bg-card',
  bgCardHover: '--bg-card-hover',
  bgGlass: '--bg-glass',
  textPrimary: '--text-primary',
  textSecondary: '--text-secondary',
  textMuted: '--text-muted',
  accent: '--accent',
  accentLight: '--accent-light',
  accentGlow: '--accent-glow',
  border: '--border',
  borderFocus: '--border-focus',
  success: '--success',
  warning: '--warning',
  danger: '--danger',
  info: '--info',
  shadowSm: '--shadow-sm',
  shadowMd: '--shadow-md',
  shadowLg: '--shadow-lg',
};

export { ThemeProvider, useTheme } from './ThemeProvider';
export { defaultDark } from './presets/default-dark';
export { defaultLight } from './presets/default-light';
export { highContrast } from './presets/high-contrast';
