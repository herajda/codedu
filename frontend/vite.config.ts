import devtoolsJson from 'vite-plugin-devtools-json';
import { sveltekit } from '@sveltejs/kit/vite';
import { defineConfig } from 'vite';
import tailwindcss from '@tailwindcss/vite';
import path from 'path';

export default defineConfig({
        plugins: [
                tailwindcss(),
                sveltekit(),
                devtoolsJson()
        ],
        resolve: {
                alias: {
                        'scratch-vm': path.resolve(__dirname, 'node_modules/scratch-vm/dist/web/scratch-vm.js'),
                        'scratch-render': path.resolve(__dirname, 'node_modules/scratch-render/dist/web/scratch-render.js'),
                        'scratch-storage': path.resolve(__dirname, 'node_modules/scratch-storage/dist/web/scratch-storage.js'),
                        'scratch-audio': path.resolve(__dirname, 'node_modules/scratch-audio/dist.js')
                }
        },
        envPrefix: ['VITE_', 'BAKALARI_'],
        worker: {
                format: 'es'
        },
        ssr: {
                worker: { format: 'es' }
	},
	server: {
    proxy: {
      '/api': {
        target: 'http://localhost:22946',
        changeOrigin: true,
        ws: true,
      },
    }
}
});
