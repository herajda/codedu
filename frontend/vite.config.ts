import devtoolsJson from 'vite-plugin-devtools-json';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';

export default defineConfig({
        plugins: [
                tailwindcss(),
                sveltekit(),
                devtoolsJson()
        ],
        envPrefix: ['VITE_', 'BAKALARI_'],
        worker: {
                format: 'es'
        },
        ssr: {
                worker: { format: 'es' }
	},
	server: {
    proxy: {
      '/api':      'http://localhost:22946',
    }
}
});
