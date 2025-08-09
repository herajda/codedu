Default Avatars
----------------

This app includes a small, built-in set of SVG avatars in `static/avatars/` which are served at runtime from `/avatars/...` by the backend. Users without an avatar automatically receive a random one on first load, and can switch to any default avatar or upload their own image in the Settings dialog.

To change or add avatars, drop additional SVG or PNG files into `static/avatars/` and rebuild the frontend.

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
