import type { LayoutServerLoad } from './$types';
import { detectLocale, ensureLocale } from '$lib/i18n/detect';
import { DEFAULT_LOCALE, SUPPORTED_LOCALES, type Locale, type TranslationDictionary } from '$lib/i18n';
import baseMessages from '$lib/i18n/locales/en.json';

const localeModules = import.meta.glob('../lib/i18n/locales/*.json');

async function loadLocaleDictionary(locale: Locale): Promise<TranslationDictionary> {
  const key = `../lib/i18n/locales/${locale}.json`;
  const loader = localeModules[key];
  if (!loader) {
    return {};
  }
  const module = (await loader()) as { default: TranslationDictionary };
  return module.default ?? {};
}

export const load: LayoutServerLoad = async ({ request, fetch }) => {
  const acceptLanguage = request.headers.get('accept-language');
  let resolvedLocale = detectLocale(acceptLanguage);

  try {
    const meRes = await fetch('/api/me');
    if (meRes.ok) {
      const me = (await meRes.json()) as { preferred_locale?: string | null };
      if (typeof me.preferred_locale === 'string' && me.preferred_locale.length > 0) {
        resolvedLocale = ensureLocale(me.preferred_locale);
      }
    }
  } catch {
    // ignore errors and fall back to detected locale
  }

  const messages = resolvedLocale === DEFAULT_LOCALE ? (baseMessages as TranslationDictionary) : await loadLocaleDictionary(resolvedLocale);

  return {
    locale: resolvedLocale,
    messages,
    fallbackMessages: baseMessages as TranslationDictionary,
    availableLocales: [...SUPPORTED_LOCALES]
  };
};
