import { browser } from '$app/environment';
import { derived, get, writable } from 'svelte/store';
import IntlMessageFormat from 'intl-messageformat';

export type Locale = 'en' | 'cs';
export type TranslationDictionary = Record<string, string>;

export const SUPPORTED_LOCALES: readonly Locale[] = ['en', 'cs'] as const;
export const DEFAULT_LOCALE: Locale = 'en';

import baseMessages from './locales/en.json';

type TemplateVariables = Record<string, string | number | boolean>;

type Translator = (key: string, vars?: TemplateVariables) => string;

const localeStore = writable<Locale>(DEFAULT_LOCALE);
const dictionaryStore = writable<TranslationDictionary>({ ...(baseMessages as TranslationDictionary) });
const fallbackStore = writable<TranslationDictionary>({ ...(baseMessages as TranslationDictionary) });

export const locale = localeStore;
export const translations = dictionaryStore;
export const fallbackTranslations = fallbackStore;

const placeholderPattern = /{{\s*([\w.-]+)\s*}}|\{\s*([\w.-]+)\s*\}/g;

function formatWithFallback(template: string, vars: TemplateVariables = {}): string {
  return template.replace(placeholderPattern, (_, doubleName, singleName) => {
    const name = (doubleName ?? singleName) as keyof TemplateVariables;
    const value = vars[name];
    return value === undefined || value === null ? '' : String(value);
  });
}

function formatTemplate(template: string, vars: TemplateVariables = {}, locale: Locale): string {
  if (!template.includes('{')) {
    return template;
  }

  try {
    const formatter = new IntlMessageFormat(template, locale);
    const result = formatter.format(vars);
    if (Array.isArray(result)) {
      return result.join('');
    }
    return typeof result === 'string' ? result : String(result);
  } catch (err) {
    return formatWithFallback(template, vars);
  }
}

export const translator = derived([localeStore, dictionaryStore, fallbackStore], ([$locale, $dictionary, $fallback]): Translator => {
  return (key, vars = {}) => {
    const template = $dictionary[key] ?? $fallback[key] ?? key;
    return formatTemplate(template, vars, $locale);
  };
});

export function translate(key: string, vars?: TemplateVariables): string {
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
