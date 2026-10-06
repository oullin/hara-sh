import { fileURLToPath } from 'node:url';
import { defineConfig } from 'vitest/config';

export default defineConfig(
	{
		resolve: {
			alias: {
				'@': fileURLToPath(new URL('./src', import.meta.url)),
				'@test': fileURLToPath(new URL('./test', import.meta.url)),
			},
		},
		test: {
			coverage: {
				include: ['src/**/*.ts'],
				provider: 'v8',
				reporter: ['text', 'html', 'json-summary'],
				thresholds: { 100: true },
			},
			environment: 'node',
			include: ['test/**/*.test.ts'],
			restoreMocks: true,
			setupFiles: ['test/setup.ts'],
		},
	},
);
