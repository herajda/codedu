// See https://svelte.dev/docs/kit/types#app.d.ts
// for information about these interfaces
declare global {
        namespace App {
                // interface Error {}
                // interface Locals {}
                interface PageData {
                        locale?: import('$lib/i18n').Locale;
                        messages?: import('$lib/i18n').TranslationDictionary;
                        fallbackMessages?: import('$lib/i18n').TranslationDictionary;
                        availableLocales?: readonly import('$lib/i18n').Locale[];
                }
                // interface PageState {}
                // interface Platform {}
        }
}

declare module 'cally';

export {};
