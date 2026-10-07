import { readFile, writeFile } from 'node:fs/promises';
import process from 'node:process';
import { fileURLToPath, URL } from 'node:url';
import vue from '@vitejs/plugin-vue';
import { createServer } from 'vite';
import { createSSRApp, type Component } from 'vue';
import { renderToString } from 'vue/server-renderer';

// Render the same components at build time so crawlers and visitors without
// JavaScript receive the full page. No proxy state or credentials are read.
const root = fileURLToPath(new URL('..', import.meta.url));
const output = new URL('../.cloudflare/output/v0/workers/default/assets/index.html', import.meta.url);

const server = await createServer(
	{
		configFile: false,
		plugins: [vue()],
		resolve: { alias: { '@': fileURLToPath(new URL('../src', import.meta.url)) } },
		root,
		server: { middlewareMode: true, ws: false },
	},
);

try {
	const { default: App } = (await server.ssrLoadModule('/src/App.vue')) as { default: Component };

	const content = await renderToString(
		createSSRApp(App),
	);

	const html = await readFile(output, 'utf8');

	const marker = '<div id="app"></div>';

	if (!html.includes(marker) || !content.includes('<h1')) {
		throw new Error('Landing prerender failed: missing app root or main heading.');
	}

	await writeFile(
		output,
		html.replace(marker, `<div id="app">${content}</div>`),
	);

	process.stdout.write('Prerendered landing page for crawlers and clients without JavaScript.\n');
} finally {
	await server.close();
}
