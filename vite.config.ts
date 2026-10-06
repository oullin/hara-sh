import { cloudflare } from '@cloudflare/vite-plugin';
import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vite';

// Vite bundles and serves the Worker (`cf dev`, `cf deploy`); the Cloudflare plugin reads
// cloudflare.config.ts and runs the Worker in workerd.
export default defineConfig(
	{
		plugins: [cloudflare()],
		resolve: {
			alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
		},
	},
);
