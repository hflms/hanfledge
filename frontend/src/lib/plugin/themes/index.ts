// -- Theme System -----------------------------------------------

export interface ThemeColors {
  bgPrimary: string;
  bgSecondary: string;
  bgTertiary: string;
  textPrimary: string;
  textSecondary: string;
  textMuted: string;
  accentColor: string;
  accentHover: string;
  borderColor: string;
  successColor: string;
  warningColor: string;
  errorColor: string;
  infoColor: string;
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

export const THEME_CSS_VAR_MAP: Record<keyof ThemeColors, string> = {
  bgPrimary: '--bg-primary',
  bgSecondary: '--bg-secondary',
  bgTertiary: '--bg-tertiary',
  textPrimary: '--text-primary',
  textSecondary: '--text-secondary',
  textMuted: '--text-muted',
  accentColor: '--accent-color',
  accentHover: '--accent-hover',
  borderColor: '--border-color',
  successColor: '--success-color',
  warningColor: '--warning-color',
  errorColor: '--error-color',
  infoColor: '--info-color',
};

export { ThemeProvider, useTheme } from './ThemeProvider';
export { defaultDark } from './presets/default-dark';
export { defaultLight } from './presets/default-light';
export { highContrast } from './presets/high-contrast';
