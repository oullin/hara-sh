import { cloudflare } from '@cloudflare/vite-plugin';
import tailwindcss from '@tailwindcss/vite';
import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vite';

// Vite builds the Vue app; the Cloudflare plugin reads cloudflare.config.ts for `cf dev` / `cf deploy`.
export default defineConfig(
	{
		plugins: [vue(), tailwindcss(), cloudflare()],
		resolve: {
			alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
		},
	},
);
