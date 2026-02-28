import type { ThemeConfig } from '../index';

export const defaultLight: ThemeConfig = {
  id: 'default-light',
  name: '默认浅色',
  description: '默认浅色主题 (Catppuccin Latte)',
  type: 'light',
  colors: {
    bgPrimary: '#eff1f5',
    bgSecondary: '#e6e9ef',
    bgTertiary: '#dce0e8',
    textPrimary: '#4c4f69',
    textSecondary: '#5c5f77',
    textMuted: '#9ca0b0',
    accentColor: '#1e66f5',
    accentHover: '#209fb5',
    borderColor: '#ccd0da',
    successColor: '#40a02b',
    warningColor: '#df8e1d',
    errorColor: '#d20f39',
    infoColor: '#04a5e5',
  },
};
