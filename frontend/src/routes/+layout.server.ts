import type { LayoutServerLoad } from './$types';
import { detectLocale } from '$lib/i18n/detect';
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

export const load: LayoutServerLoad = async ({ request }) => {
  const acceptLanguage = request.headers.get('accept-language');
  const resolvedLocale = detectLocale(acceptLanguage);

  const messages = resolvedLocale === DEFAULT_LOCALE ? (baseMessages as TranslationDictionary) : await loadLocaleDictionary(resolvedLocale);

  return {
    locale: resolvedLocale,
    messages,
    fallbackMessages: baseMessages as TranslationDictionary,
    availableLocales: [...SUPPORTED_LOCALES]
  };
};
