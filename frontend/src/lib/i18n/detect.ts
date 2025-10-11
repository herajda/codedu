import { DEFAULT_LOCALE, SUPPORTED_LOCALES, type Locale } from './index';

type WeightedLocale = {
  tag: string;
  weight: number;
};

function parseAcceptLanguage(header: string | null | undefined): WeightedLocale[] {
  if (!header) return [];

  return header
    .split(',')
    .map((part) => {
      const [tagPart, weightPart] = part.trim().split(';');
      const weight = weightPart?.startsWith('q=') ? Number.parseFloat(weightPart.slice(2)) : 1;
      return { tag: tagPart.toLowerCase(), weight: Number.isFinite(weight) ? weight : 0 } satisfies WeightedLocale;
    })
    .filter((entry) => entry.weight > 0)
    .sort((a, b) => b.weight - a.weight);
}

function normalize(tag: string): string {
  return tag.toLowerCase().replace('_', '-');
}

export function detectLocale(header: string | null | undefined): Locale {
  const weighted = parseAcceptLanguage(header);

  for (const entry of weighted) {
    const primary = normalize(entry.tag).split('-')[0];
    const exact = SUPPORTED_LOCALES.find((locale) => normalize(locale) === normalize(entry.tag));
    if (exact) return exact;

    const partial = SUPPORTED_LOCALES.find((locale) => locale.startsWith(primary as Locale));
    if (partial) return partial;
  }

  return DEFAULT_LOCALE;
}

export function ensureLocale(value: string | null | undefined): Locale {
  if (!value) return DEFAULT_LOCALE;
  const normalized = value.toLowerCase();
  return (SUPPORTED_LOCALES.find((locale) => locale === normalized) ?? DEFAULT_LOCALE) as Locale;
}
