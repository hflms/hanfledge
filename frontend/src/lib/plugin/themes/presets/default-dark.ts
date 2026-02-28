import type { ThemeConfig } from '../index';

export const defaultDark: ThemeConfig = {
  id: 'default-dark',
  name: '默认深色',
  description: '默认深色主题 (Catppuccin Mocha)',
  type: 'dark',
  colors: {
    bgPrimary: '#11111b',
    bgSecondary: '#1e1e2e',
    bgTertiary: '#2a2a3e',
    textPrimary: '#cdd6f4',
    textSecondary: '#bac2de',
    textMuted: '#6c7086',
    accentColor: '#89b4fa',
    accentHover: '#74c7ec',
    borderColor: '#313244',
    successColor: '#a6e3a1',
    warningColor: '#f9e2af',
    errorColor: '#f38ba8',
    infoColor: '#89dceb',
  },
};
