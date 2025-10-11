import { browser } from '$app/environment';
import { derived, get, writable } from 'svelte/store';

export type Locale = 'en' | 'cs';
export type TranslationDictionary = Record<string, string>;

export const SUPPORTED_LOCALES: readonly Locale[] = ['en', 'cs'] as const;
export const DEFAULT_LOCALE: Locale = 'en';

import baseMessages from './locales/en.json';

type Translator = (key: string, vars?: Record<string, string | number>) => string;

const localeStore = writable<Locale>(DEFAULT_LOCALE);
const dictionaryStore = writable<TranslationDictionary>({ ...(baseMessages as TranslationDictionary) });
const fallbackStore = writable<TranslationDictionary>({ ...(baseMessages as TranslationDictionary) });

export const locale = localeStore;
export const translations = dictionaryStore;
export const fallbackTranslations = fallbackStore;

const placeholderPattern = /{{\s*([\w.-]+)\s*}}|\{\s*([\w.-]+)\s*\}/g;

function formatTemplate(template: string, vars: Record<string, string | number> = {}): string {
  return template.replace(placeholderPattern, (_, doubleName, singleName) => {
    const name = doubleName ?? singleName;
    const value = vars[name];
    return value === undefined || value === null ? '' : String(value);
  });
}

export const translator = derived([dictionaryStore, fallbackStore], ([$dictionary, $fallback]): Translator => {
  return (key, vars = {}) => {
    const template = $dictionary[key] ?? $fallback[key] ?? key;
    return formatTemplate(template, vars);
  };
});

export function translate(key: string, vars?: Record<string, string | number>): string {
  return get(translator)(key, vars);
}

export const t = translate;

export function applyRuntimeI18n(locale: Locale, bundle: TranslationDictionary, fallback: TranslationDictionary): void {
  localeStore.set(locale);
  dictionaryStore.set(bundle);
  fallbackStore.set(fallback);

  if (browser) {
    document.documentElement.lang = locale;
  }
}

export function mergeFallbackTranslations(additional: TranslationDictionary): void {
  fallbackStore.update((existing) => ({ ...existing, ...additional }));
}

export type { Translator };
