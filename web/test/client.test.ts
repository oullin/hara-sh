import { describe, expect, it, vi } from 'vitest';
import { SemanticSearch } from '@/../docs/.vitepress/theme/search';
import { DOCS_HOST } from '@/../worker/host';
import worker from '@/../worker/index';
import { search } from './fakes';

// Sends the docs client's requests to the real Worker handler with in-memory bindings.
function client() {
	const bindings = { ...search(), ASSETS: { fetch: vi.fn(() => Promise.resolve(new Response('asset'))) } };
	const semantic = new SemanticSearch((input, init) => Promise.resolve(worker.fetch(new Request(`https://${DOCS_HOST}${input}`, init), bindings)));

	return { bindings, semantic };
}

describe('docs search client', () => {
	it('returns Worker results for a question', async () => {
		const { bindings, semantic } = client();

		await worker.fetch(new Request(`https://${DOCS_HOST}/api/search/index`, { headers: { authorization: 'Bearer test-token' }, method: 'POST' }), bindings);

		const results = await semantic.query('rotate the client key');

		expect(results).toHaveLength(5);
		expect(results![0]).toEqual(expect.objectContaining({ score: expect.any(Number), title: expect.any(String), url: expect.stringMatching(/^\//) }));
	});

	it('returns null when the Worker fails, so the UI opens local search', async () => {
		const { bindings, semantic } = client();

		bindings.AI.failing = true;

		expect(await semantic.query('keys')).toBeNull();
	});

	it('returns null when the API cannot be reached', async () => {
		const semantic = new SemanticSearch(() => Promise.reject(new TypeError('Failed to fetch')));

		expect(await semantic.query('keys')).toBeNull();
	});
});
