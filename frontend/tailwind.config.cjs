module.exports = {
  content: [
    './index.html',
    './src/**/*.{svelte,js,ts}'
  ],
  theme: {
    extend: {
      fontFamily: {
        sans: ['Inter', 'sans-serif'],
      },
    },
  },
  plugins: [require('@tailwindcss/typography')],
}
