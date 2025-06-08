// frontend/vite.config.ts
import { defineConfig } from 'vite'
import { svelte } from '@sveltejs/vite-plugin-svelte'

export default defineConfig({
  plugins: [svelte()],
  server: {
    proxy: {
      '/api': 'http://localhost:22946',
      '/register': 'http://localhost:22946',
      '/login': 'http://localhost:22946'
    }
  }
})
