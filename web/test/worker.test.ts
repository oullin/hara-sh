import { describe, expect, it, vi } from 'vitest';
import { CANONICAL_HOST, DOCS_HOST } from '@/../worker/host';
import worker from '@/../worker/index';

function env() {
	return { ASSETS: { fetch: vi.fn((request: Request) => Promise.resolve(new Response(`asset ${new URL(request.url).pathname}`))) } };
}

describe('landing page worker', () => {
	it.each([
		['http://hara.sh/?source=phone', 'https://hara.sh/?source=phone', 308],
		['http://docs.hara.sh/setup', 'https://docs.hara.sh/setup', 308],
		['http://www.hara.sh/', 'https://hara.sh/', 301],
		['https://hara.sh/index.html?x=1', 'https://hara.sh/?x=1', 308],
	])('redirects %s to its canonical origin or landing URL', async (source, target, status) => {
		const assets = env();

		const response = await worker.fetch(new Request(source), assets);

		expect(response.status).toBe(status);
		expect(
			response.headers.get('location'),
		).toBe(target);
		expect(assets.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it.each([CANONICAL_HOST, DOCS_HOST])('keeps missing pages on %s out of search results', async (host) => {
		const assets = { ASSETS: { fetch: vi.fn(() => Promise.resolve(new Response('Missing', { status: 404 }))) } };

		const response = await worker.fetch(new Request(`https://${host}/does-not-exist`), assets);

		expect(response.status).toBe(404);
		expect(
			response.headers.get('X-Robots-Tag'),
		).toBe('noindex');

		expect(await response.text()).toBe('Missing');
	});

	it('serves local HTTP development without redirecting to HTTPS', async () => {
		const assets = env();

		const response = await worker.fetch(new Request('http://localhost:5173/'), assets);

		expect(response.status).toBe(200);

		expect(await response.text()).toBe('asset /index.html');
	});

	it('redirects www to the apex, keeping path and query', async () => {
		const assets = env();

		const res = await worker.fetch(new Request('https://www.hara.sh/setup?x=1'), assets);

		expect(res.status).toBe(301);
		expect(
			res.headers.get('location'),
		).toBe(`https://${CANONICAL_HOST}/setup?x=1`);
		expect(assets.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it('serves static assets on the apex', async () => {
		const assets = env();

		const res = await worker.fetch(new Request('https://hara.sh/robots.txt'), assets);

		expect(await res.text()).toBe('asset /robots.txt');

		expect(assets.ASSETS.fetch).toHaveBeenCalledOnce();
	});

	it('serves the landing index when HTML rewriting is disabled', async () => {
		const assets = env();

		const response = await worker.fetch(new Request(`https://${CANONICAL_HOST}/?source=phone`), assets);

		expect(await response.text()).toBe('asset /index.html');

		expect(new URL(assets.ASSETS.fetch.mock.calls[0]![0].url).search).toBe('?source=phone');
	});

	it.each([
		['/', '/docs/index.html'],
		['/setup', '/docs/setup.html'],
		['/assets/app.js', '/docs/assets/app.js'],
		['/favicon.svg', '/docs/favicon.svg'],
		['/sitemap.xml', '/docs/sitemap.xml'],
		['/missing-page', '/docs/missing-page.html'],
	])('serves docs request %s from its own asset tree', async (path, expected) => {
		const assets = env();

		const response = await worker.fetch(new Request(`https://${DOCS_HOST}${path}?x=1`), assets);

		expect(await response.text()).toBe(`asset ${expected}`);

		expect(new URL(assets.ASSETS.fetch.mock.calls[0]![0].url).search).toBe('?x=1');
	});

	it.each([
		['/index.html', '/'],
		['/setup.html', '/setup'],
		['/setup/', '/setup'],
	])('redirects noncanonical docs path %s to %s', async (path, expected) => {
		const assets = env();

		const response = await worker.fetch(new Request(`https://${DOCS_HOST}${path}?x=1`), assets);

		expect(response.status).toBe(308);
		expect(
			response.headers.get('location'),
		).toBe(`https://${DOCS_HOST}${expected}?x=1`);
		expect(assets.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it.each([
		['/docs', '/'],
		['/docs/', '/'],
		['/docs/setup', '/setup'],
	])('redirects old documentation path %s to its subdomain', async (path, expected) => {
		const assets = env();

		const response = await worker.fetch(new Request(`https://${CANONICAL_HOST}${path}?x=1`), assets);

		expect(response.status).toBe(308);
		expect(
			response.headers.get('location'),
		).toBe(`https://${DOCS_HOST}${expected}?x=1`);
		expect(assets.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it.each([
		['/docs/assets/chunks/%40search.js?x=1', '/assets/chunks/%40search.js?x=1'],
		['/docs', '/'],
	])('keeps internal asset redirects out of public docs URLs', async (location, expected) => {
		const assets = {
			ASSETS: { fetch: vi.fn(() => Promise.resolve(new Response(null, { headers: { location }, status: 307 }))) },
		};

		const response = await worker.fetch(new Request(`https://${DOCS_HOST}/assets/chunks/@search.js`), assets);

		expect(response.status).toBe(307);
		expect(
			response.headers.get('location'),
		).toBe(`https://${DOCS_HOST}${expected}`);
	});
});
