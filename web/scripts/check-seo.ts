import assert from 'node:assert/strict';
import { readFile, readdir } from 'node:fs/promises';
import process from 'node:process';
import { URL } from 'node:url';
import { JSDOM } from 'jsdom';

const guides = (await readdir(new URL('.', import.meta.resolve('#docs/index.md')))).filter((file) => file.endsWith('.md'));

const pages = [
	{
		file: 'index.html',
		url: 'https://hara.sh/',
	},
	...guides.map((file) => ({
		file: `docs/${file.replace(/\.md$/, '.html')}`,
		url: `https://docs.hara.sh/${file === 'index.md' ? '' : file.replace(/\.md$/, '')}`,
	})),
];

const descriptions = new Set<string>();
const titles = new Set<string>();

for (const { file, url } of pages) {
	const dom = new JSDOM(await readFile(new URL(import.meta.resolve(`#assets/${file}`)), 'utf8'));

	const { document } = dom.window;
	const meta = (selector: string): string | null | undefined => document.querySelector(selector)?.getAttribute('content');
	const title = document.title;
	const description = meta('meta[name="description"]');

	assert.equal(document.documentElement.lang, 'en-GB', `${file}: language`);
	assert(title.length >= 15 && title.length <= 70, `${file}: title length`);
	assert(!titles.has(title), `${file}: duplicate title`);
	assert(description && description.length >= 70 && description.length <= 170, `${file}: description length`);
	assert(!descriptions.has(description), `${file}: duplicate description`);
	assert.equal(document.querySelectorAll('h1').length, 1, `${file}: main heading missing from HTML`);
	assert.equal(document.querySelectorAll('link[rel="canonical"]').length, 1, `${file}: canonical count`);
	assert.equal(document.querySelector('link[rel="canonical"]')?.getAttribute('href'), url, `${file}: canonical`);
	assert.equal(meta('meta[property="og:url"]'), url, `${file}: social URL`);
	assert.equal(meta('meta[property="og:title"]'), title, `${file}: social title`);
	assert.equal(meta('meta[property="og:locale"]'), 'en_GB', `${file}: locale`);
	assert.equal(meta('meta[name="twitter:card"]'), 'summary_large_image', `${file}: social card`);
	assert.equal(meta('meta[name="twitter:title"]'), title, `${file}: Twitter title`);

	for (const selector of ['meta[property="og:description"]', 'meta[name="twitter:description"]', 'meta[property="og:image:alt"]', 'meta[name="twitter:image:alt"]']) {
		assert(
			meta(selector),
			`${file}: missing ${selector}`,
		);
	}

	assert.equal(meta('meta[property="og:image"]'), 'https://hara.sh/og.png', `${file}: preview image`);
	assert.equal(meta('meta[name="twitter:image"]'), 'https://hara.sh/og.png', `${file}: Twitter image`);
	assert.match(meta('meta[name="robots"]') ?? '', /^index, follow/, `${file}: indexing`);

	const schemas = Array.from(document.querySelectorAll('script[type="application/ld+json"]'));

	assert(schemas.length, `${file}: structured data`);
	assert(
		schemas.every((script) => {
			const schema: unknown = JSON.parse(script.textContent ?? '');

			return typeof schema === 'object' && schema !== null && '@context' in schema && schema['@context'] === 'https://schema.org';
		}),
		`${file}: schema context`,
	);
	assert(!/\b(?:center|behavior|recognizable|authorize|favors|sanitized|containerized)\b/i.test(document.body.textContent ?? ''), `${file}: British English`);
	descriptions.add(description);
	titles.add(title);
	dom.window.close();
}

for (const [file, origin] of [
	['sitemap.xml', 'https://hara.sh'],
	['docs/sitemap.xml', 'https://docs.hara.sh'],
] as const) {
	const dom = new JSDOM(
		await readFile(new URL(import.meta.resolve(`#assets/${file}`)), 'utf8'),
		{ contentType: 'application/xml' },
	);

	const urls = Array.from(dom.window.document.querySelectorAll('loc'), (node) => node.textContent ?? '');

	assert.deepEqual(urls.toSorted(), pages.flatMap(({ url }) => (new URL(url).origin === origin ? [url] : [])).toSorted(), `${file}: sitemap routes`);
	dom.window.close();
}

const missing = new JSDOM(await readFile(new URL(import.meta.resolve('#assets/docs/404.html')), 'utf8'));

assert.match(missing.window.document.querySelector('meta[name="robots"]')?.getAttribute('content') ?? '', /noindex/, '404 must not be indexed');
assert.equal(missing.window.document.querySelector('link[rel="canonical"]'), null, '404 must not claim a canonical page');
missing.window.close();
process.stdout.write(`SEO checks passed for ${pages.length} rendered pages, both sitemaps and the documentation 404.\n`);
