Default Avatars
----------------

This app includes a small, built-in set of SVG avatars in `static/avatars/` which are served at runtime from `/avatars/...` by the backend. Users without an avatar automatically receive a random one on first load, and can switch to any default avatar or upload their own image in the Settings dialog.

To change or add avatars, drop additional SVG or PNG files into `static/avatars/` and rebuild the frontend.

## Internationalization

CodEdu ships with an extensible i18n layer. Browser `Accept-Language` headers (or the `codex_locale` cookie) decide which locale is used at runtime. Strings live in `src/lib/i18n/locales/en.json`; consume them via the `$t('namespace.key')` helper exported from `$lib/i18n`.

To create or refresh a locale file, run the translation helper from the repository root:

```bash
OPENAI_API_KEY=sk-... scripts/translate_app.py \
  --source frontend/src/lib/i18n/locales/en.json \
  --target frontend/src/lib/i18n/locales/cs.json \
  --locale cs
```

Add `--all` to retranslate every key or `--dry-run` to inspect the generated prompt before calling the API. The script only overwrites keys that are missing or still match the English source.

# sv

Everything you need to build a Svelte project, powered by [`sv`](https://github.com/sveltejs/cli).

## Creating a project

If you're seeing this, you've probably already done this step. Congrats!

```bash
# create a new project in the current directory
npx sv create

# create a new project in my-app
npx sv create my-app
```

## Developing

Once you've created a project and installed dependencies with `npm install` (or `pnpm install` or `yarn`), start a development server:

```bash
npm run dev

# or start the server and open the app in a new browser tab
npm run dev -- --open
```

## Building

To create a production version of your app:

```bash
npm run build
```

You can preview the production build with `npm run preview`.

> To deploy your app, you may need to install an [adapter](https://svelte.dev/docs/kit/adapters) for your target environment.
