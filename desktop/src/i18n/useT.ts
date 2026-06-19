import { useStore } from '../store';
import { translate, type TranslationKey } from './i18n';

export type TFunc = (
  key: TranslationKey,
  vars?: Record<string, string | number>,
) => string;

export function useT(): TFunc {
  const lang = useStore((s) => s.lang);
  return (key, vars) => translate(lang, key, vars);
}
