/** @type {import('tailwindcss').Config} */
export default {
  content: ['./src/**/*.{svelte,html,ts,js}'],
  theme: { extend: {} },
  plugins: [
    require('@tailwindcss/forms'),
    require('@tailwindcss/typography')
  ]
};
