'use client';

import { createContext, useCallback, useContext, useEffect, useState, type ReactNode } from 'react';
import type { ThemeColors, ThemeConfig } from './index';
import { THEME_CSS_VAR_MAP } from './index';
import { defaultDark } from './presets/default-dark';
import { defaultLight } from './presets/default-light';
import { highContrast } from './presets/high-contrast';
import styles from './ThemeProvider.module.css';

// -- Built-in Themes --------------------------------------------

const BUILT_IN_THEMES: ThemeConfig[] = [defaultDark, defaultLight, highContrast];

// -- Theme Context ----------------------------------------------

interface ThemeContextValue {
  theme: ThemeConfig;
  themes: ThemeConfig[];
  setTheme: (themeId: string) => void;
  registerTheme: (theme: ThemeConfig) => void;
}

const ThemeContext = createContext<ThemeContextValue | null>(null);

// -- Apply Theme to DOM -----------------------------------------

function applyTheme(theme: ThemeConfig) {
  const root = document.documentElement;

  // Apply CSS variables
  for (const [key, cssVar] of Object.entries(THEME_CSS_VAR_MAP)) {
    const value = theme.colors[key as keyof ThemeColors];
    if (value) {
      root.style.setProperty(cssVar, value);
    }
  }

  // Apply data attribute for CSS selectors
  root.setAttribute('data-theme', theme.id);
  root.setAttribute('data-theme-type', theme.type);

  // Apply custom CSS if provided
  let customStyleEl = document.getElementById('hanfledge-theme-custom');
  if (theme.customCSS) {
    if (!customStyleEl) {
      customStyleEl = document.createElement('style');
      customStyleEl.id = 'hanfledge-theme-custom';
      document.head.appendChild(customStyleEl);
    }
    customStyleEl.textContent = theme.customCSS;
  } else if (customStyleEl) {
    customStyleEl.remove();
  }
}

// -- Provider Component -----------------------------------------

interface ThemeProviderProps {
  children: ReactNode;
  defaultTheme?: string;
}

export function ThemeProvider({ children, defaultTheme }: ThemeProviderProps) {
  const [themes, setThemes] = useState<ThemeConfig[]>(BUILT_IN_THEMES);
  const [theme, setCurrentTheme] = useState<ThemeConfig>(BUILT_IN_THEMES[0]);

  // Load saved preference
  useEffect(() => {
    const saved = localStorage.getItem('hanfledge_theme');
    const themeId = saved || defaultTheme || 'default-dark';
    const found = themes.find((t) => t.id === themeId);
    if (found) {
      setTimeout(() => setCurrentTheme(found), 0);
      applyTheme(found);
    }
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const setTheme = useCallback(
    (themeId: string) => {
      const found = themes.find((t) => t.id === themeId);
      if (found) {
        setCurrentTheme(found);
        applyTheme(found);
        localStorage.setItem('hanfledge_theme', themeId);
      }
    },
    [themes],
  );

  const registerTheme = useCallback((newTheme: ThemeConfig) => {
    setThemes((prev) => {
      // Replace if same ID exists, otherwise append
      const exists = prev.findIndex((t) => t.id === newTheme.id);
      if (exists >= 0) {
        const updated = [...prev];
        updated[exists] = newTheme;
        return updated;
      }
      return [...prev, newTheme];
    });
  }, []);

  return (
    <ThemeContext.Provider value={{ theme, themes, setTheme, registerTheme }}>
      <div className={styles.themeRoot}>{children}</div>
    </ThemeContext.Provider>
  );
}

export function useTheme() {
  const ctx = useContext(ThemeContext);
  if (!ctx) {
    throw new Error('useTheme must be used within a ThemeProvider');
  }
  return ctx;
}
