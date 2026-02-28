// -- i18n -- Frontend Internationalization ----------------------

export type Locale = 'zh-CN' | 'en-US';

export const DEFAULT_LOCALE: Locale = 'zh-CN';
export const SUPPORTED_LOCALES: Locale[] = ['zh-CN', 'en-US'];

// Locale display names
export const LOCALE_NAMES: Record<Locale, string> = {
  'zh-CN': '简体中文',
  'en-US': 'English',
};

type Messages = Record<string, string>;

const messageCache: Partial<Record<Locale, Messages>> = {};

/**
 * Load locale messages dynamically.
 */
export async function loadMessages(locale: Locale): Promise<Messages> {
  if (messageCache[locale]) {
    return messageCache[locale]!;
  }

  try {
    const mod = await import(`./messages/${locale}.json`);
    const messages = mod.default as Messages;
    messageCache[locale] = messages;
    return messages;
  } catch {
    console.warn(`[i18n] Failed to load locale: ${locale}, falling back to ${DEFAULT_LOCALE}`);
    if (locale !== DEFAULT_LOCALE) {
      return loadMessages(DEFAULT_LOCALE);
    }
    return {};
  }
}

/**
 * Get the stored locale preference, or detect from browser.
 */
export function getLocale(): Locale {
  // Check localStorage
  if (typeof window !== 'undefined') {
    const stored = localStorage.getItem('hanfledge_locale') as Locale;
    if (stored && SUPPORTED_LOCALES.includes(stored)) {
      return stored;
    }

    // Detect from browser
    const browserLang = navigator.language;
    for (const locale of SUPPORTED_LOCALES) {
      if (browserLang.startsWith(locale.split('-')[0])) {
        return locale;
      }
    }
  }

  return DEFAULT_LOCALE;
}

/**
 * Save locale preference.
 */
export function setLocale(locale: Locale): void {
  if (typeof window !== 'undefined') {
    localStorage.setItem('hanfledge_locale', locale);
  }
}
