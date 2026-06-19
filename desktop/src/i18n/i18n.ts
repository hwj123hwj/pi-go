import { zh } from './locales/zh';
import { en } from './locales/en';

export type Lang = 'zh' | 'en';

export type TranslationKey = keyof typeof zh;

const catalogs: Record<Lang, Record<string, string>> = { zh, en };

const STORAGE_KEY = 'pigo.lang';

export function detectSystemLang(): Lang {
  const nav =
    typeof navigator !== 'undefined' ? navigator.language || '' : '';
  return nav.toLowerCase().startsWith('zh') ? 'zh' : 'en';
}

export function loadStoredLang(): Lang {
  try {
    const v = localStorage.getItem(STORAGE_KEY);
    if (v === 'zh' || v === 'en') return v;
  } catch {
    /* localStorage unavailable */
  }
  return detectSystemLang();
}

export function persistLang(lang: Lang): void {
  try {
    localStorage.setItem(STORAGE_KEY, lang);
  } catch {
    /* best-effort */
  }
}

export function translate(
  lang: Lang,
  key: TranslationKey,
  vars?: Record<string, string | number>,
): string {
  const template =
    catalogs[lang]?.[key] ?? catalogs.zh[key] ?? (key as string);
  if (!vars) return template;
  return template.replace(/\{(\w+)\}/g, (_, name: string) =>
    name in vars ? String(vars[name]) : `{${name}}`,
  );
}
