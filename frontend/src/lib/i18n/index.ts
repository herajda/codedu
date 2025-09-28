import { browser } from '$app/environment';
import { init, locale as localeStore, register, waitLocale } from 'svelte-i18n';

import { DEFAULT_LOCALE, FALLBACK_LOCALE, SUPPORTED_LOCALES, type AppLocale } from './config';

let registered = false;
let initialized = false;
let currentLocale: AppLocale | null = null;

function ensureRegistration() {
  if (registered) return;

  register('en', () => import('./locales/en.json'));
  register('cs', () => import('./locales/cs.json'));

  registered = true;
}

function normalize(locale: string | null | undefined): AppLocale | null {
  if (!locale) return null;
  const lower = locale.toLowerCase().replace('_', '-');
  if (SUPPORTED_LOCALES.includes(lower as AppLocale)) {
    return lower as AppLocale;
  }
  const language = lower.split('-')[0];
  if (SUPPORTED_LOCALES.includes(language as AppLocale)) {
    return language as AppLocale;
  }
  return null;
}

export function setupI18n(initialLocale?: string | null) {
  ensureRegistration();

  const targetLocale = normalize(initialLocale) ?? DEFAULT_LOCALE;

  if (!initialized) {
    init({
      fallbackLocale: FALLBACK_LOCALE,
      initialLocale: targetLocale,
    });
    initialized = true;
  }

  if (currentLocale !== targetLocale) {
    localeStore.set(targetLocale);
    currentLocale = targetLocale;
  }

  return waitLocale();
}

export async function changeLocale(nextLocale: string) {
  const normalized = normalize(nextLocale);
  if (!normalized) {
    throw new Error(`Unsupported locale: ${nextLocale}`);
  }
  ensureRegistration();
  localeStore.set(normalized);
  currentLocale = normalized;
  await waitLocale();

  if (browser) {
    document.documentElement.lang = normalized;
  }
}

export { locale as localeStore } from 'svelte-i18n';
