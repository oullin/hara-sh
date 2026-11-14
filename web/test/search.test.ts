import { describe, expect, it, vi } from 'vitest';
import { DOCS_HOST } from '@/../worker/host';
import worker from '@/../worker/index';
import { chunks, documents } from '@/../worker/docs';
import { Search } from '@/../worker/search';
import { search } from './fakes';

function env() {
	return { ...search(), ASSETS: { fetch: vi.fn(() => Promise.resolve(new Response('asset'))) } };
}

function post(path: string, body: string, headers: Record<string, string> = {}) {
	return new Request(`https://${DOCS_HOST}${path}`, { body, headers, method: 'POST' });
}

function index(token = 'test-token') {
	return post(
		'/api/search/index',
		'',
		{ authorization: `Bearer ${token}` },
	);
}

function query(q: unknown) {
	return post(
		'/api/search/query',
		JSON.stringify({ q }),
	);
}

describe('docs search', () => {
	it('indexes every docs page and finds the one a question is about', async () => {
		const bindings = env();

		const indexed = await worker.fetch(index(), bindings);

		expect(indexed.status).toBe(200);

		expect(await indexed.json()).toEqual({ pages: 11, vectors: 11 });
		// Ten chunks per embedding request.
		expect(
			bindings.AI.inputs.map((input) => input.length),
		).toEqual([10, 1]);
		expect(bindings.AI.inputs[0]![0]).toMatch(/^title: .+ \| text: # /);
		expect(bindings.VECTORIZE.vectors.get('/networking#0')?.metadata).toEqual({ title: 'Network and TLS', url: '/networking' });

		const response = await worker.fetch(query('  Tailscale HTTPS on my tailnet  '), bindings);

		const results: Array<{ score: number; title: string; url: string }> = await response.json();

		expect(response.status).toBe(200);
		expect(
			bindings.AI.inputs.at(-1),
		).toEqual(['task: search result | query: Tailscale HTTPS on my tailnet']);
		expect(results).toHaveLength(5);
		expect(results[0]).toMatchObject({ title: 'Network and TLS', url: '/networking' });
		expect(results[0]!.score).toBeGreaterThan(results[1]!.score);
		expect(new Set(results.map(({ url }) => url)).size).toBe(5);
		expect(bindings.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it('keeps the best chunk of each page', async () => {
		const bindings = search();
		const engine = new Search(bindings.AI, bindings.VECTORIZE);

		await engine.index([
			{ id: '/a#0', text: 'proxy keys', title: 'A', url: '/a' },
			{ id: '/a#1', text: 'proxy keys rotate', title: 'A', url: '/a' },
			{ id: '/b#0', text: 'proxy', title: 'B', url: '/b' },
		]);

		expect(
			(await engine.query('rotate proxy keys')).map(({ url }) => url),
		).toEqual(['/a', '/b']);
	});

	it.each([
		['a wrong token', index('wrong')],
		['no token', post('/api/search/index', '')],
	])('refuses to index with %s', async (_, request) => {
		const bindings = env();

		const response = await worker.fetch(request, bindings);

		expect(response.status).toBe(401);
		expect(bindings.VECTORIZE.vectors.size).toBe(0);
	});

	it('refuses to index when the Worker has no token set', async () => {
		const bindings = { ...env(), INDEX_TOKEN: undefined };

		const response = await worker.fetch(index('undefined'), bindings);

		expect(response.status).toBe(401);
		expect(bindings.VECTORIZE.vectors.size).toBe(0);
	});

	it.each([
		['invalid JSON', post('/api/search/query', '{')],
		['no q', post('/api/search/query', '{}')],
		['a blank q', query('   ')],
		['a q that is not text', query(42)],
		['a q over 500 characters', query('a'.repeat(501))],
	])('rejects a query with %s', async (_, request) => {
		const bindings = env();

		const response = await worker.fetch(request, bindings);

		expect(response.status).toBe(400);
		expect(bindings.AI.inputs).toEqual([]);
	});

	it('only accepts POST', async () => {
		const response = await worker.fetch(new Request(`https://${DOCS_HOST}/api/search/query?q=keys`), env());

		expect(response.status).toBe(405);
		expect(
			response.headers.get('allow'),
		).toBe('POST');
	});

	it('answers unknown search paths with 404 instead of a docs page', async () => {
		const bindings = env();

		const response = await worker.fetch(post('/api/search/other', '{}'), bindings);

		expect(response.status).toBe(404);
		expect(bindings.ASSETS.fetch).not.toHaveBeenCalled();
	});

	it('reports 502 when Workers AI fails, so the docs fall back to local search', async () => {
		const bindings = env();

		bindings.AI.failing = true;

		const response = await worker.fetch(query('keys'), bindings);

		expect(response.status).toBe(502);

		expect(await response.json()).toEqual({ error: 'Search is unavailable.' });
	});
});

describe('docs chunks', () => {
	it('keeps short pages whole and splits long ones on paragraph breaks', () => {
		expect(
			chunks(''),
		).toEqual([]);
		expect(
			chunks('one\n\ntwo', 20),
		).toEqual(['one\n\ntwo']);
		expect(
			chunks('aaaa\n\nbbbb\n\ncccc', 10),
		).toEqual(['aaaa\n\nbbbb', 'cccc']);
		expect(
			chunks('abcdefghij', 4),
		).toEqual(['abcd', 'efgh', 'ij']);
	});

	it('takes the title from the first heading and the URL from the file name', () => {
		expect(
			documents(
				{ '../docs/index.md': '---\ndescription: x\n---\n\n# Overview\n\nBody' },
			),
		).toEqual([
			{ id: '/#0', text: 'title: Overview | text: # Overview\n\nBody', title: 'Overview', url: '/' },
		]);
		expect(documents({ '../docs/notes.md': 'No heading' })[0]).toMatchObject({ title: 'notes', url: '/notes' });
	});
});
