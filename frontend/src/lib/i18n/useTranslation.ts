'use client';

import { useCallback, useEffect, useState } from 'react';
import { type Locale, DEFAULT_LOCALE, getLocale, loadMessages, setLocale } from './index';

type Messages = Record<string, string>;

/**
 * React hook for translations.
 *
 * Usage:
 *   const { t, locale, changeLocale } = useTranslation();
 *   t('course.not_found') // => "课程不存在"
 */
export function useTranslation() {
  const [locale, setCurrentLocale] = useState<Locale>(DEFAULT_LOCALE);
  const [messages, setMessages] = useState<Messages>({});

  useEffect(() => {
    const detected = getLocale();
    setCurrentLocale(detected);
    loadMessages(detected).then(setMessages);
  }, []);

  const t = useCallback(
    (key: string, ...args: (string | number)[]): string => {
      let msg = messages[key] || key;
      // Simple positional replacement: {0}, {1}, ...
      args.forEach((arg, i) => {
        msg = msg.replace(`{${i}}`, String(arg));
      });
      return msg;
    },
    [messages],
  );

  const changeLocale = useCallback(async (newLocale: Locale) => {
    setLocale(newLocale);
    const msgs = await loadMessages(newLocale);
    setMessages(msgs);
    setCurrentLocale(newLocale);
  }, []);

  return { t, locale, changeLocale };
}
