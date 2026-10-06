import vue from '@vitejs/plugin-vue';
import { fileURLToPath, URL } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig({
	plugins: [vue()],
	resolve: {
		alias: { '@': fileURLToPath(new URL('./src', import.meta.url)) },
	},
	test: {
		coverage: {
			// shadcn-vue components under ui/ are vendored; main.ts only mounts the app.
			exclude: ['src/components/ui/**', 'src/main.ts'],
			include: ['src/**/*.{ts,vue}', 'worker/**/*.ts'],
			provider: 'v8',
			reporter: ['text', 'json-summary'],
			thresholds: { 100: true },
		},
		environment: 'jsdom',
		include: ['test/**/*.test.ts'],
		restoreMocks: true,
	},
});
