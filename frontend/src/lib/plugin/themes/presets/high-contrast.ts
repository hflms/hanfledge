import type { ThemeConfig } from '../index';

export const highContrast: ThemeConfig = {
  id: 'high-contrast',
  name: '高对比度',
  description: '无障碍高对比度主题，适合视觉辅助需求',
  type: 'high-contrast',
  colors: {
    bgPrimary: '#000000',
    bgSecondary: '#1a1a1a',
    bgTertiary: '#2d2d2d',
    textPrimary: '#ffffff',
    textSecondary: '#e0e0e0',
    textMuted: '#b0b0b0',
    accentColor: '#00d4ff',
    accentHover: '#ffff00',
    borderColor: '#ffffff',
    successColor: '#00ff00',
    warningColor: '#ffff00',
    errorColor: '#ff0000',
    infoColor: '#00d4ff',
  },
};
