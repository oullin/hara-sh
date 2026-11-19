import { type Chunk, documents } from '@worker/docs.ts';
import type { Nullable, Result } from '@worker/types.ts';

export const MODEL = '@cf/google/embeddinggemma-2';

const LIMIT = 5;

const BATCH = 10;

const MAX_QUERY = 500;

const MAX_QUERY_BODY = 4096;

type Meta = { title: string; url: string };

type Embeddings = { data: Array<{ embedding: Array<number> }> };

// Structural slices of the Workers AI and Vectorize bindings, so tests can pass in-memory versions.
export type Embedder = { run(model: string, inputs: { input: Array<string> }): Promise<unknown> };

export type Store = {
	query(vector: Array<number>, options: { returnMetadata: 'all'; topK: number }): Promise<{ matches: Array<{ metadata?: object; score: number }> }>;
	upsert(vectors: Array<{ id: string; metadata: Meta; values: Array<number> }>): Promise<unknown>;
};

type Limiter = { limit(options: { key: string }): Promise<{ success: boolean }> };

export type SearchEnv = { AI: Embedder; INDEX_TOKEN?: string; SEARCH_BUDGET: Limiter; SEARCH_LIMIT: Limiter; VECTORIZE: Store };

export class Search {
	readonly #ai: Embedder;

	readonly #store: Store;

	constructor(ai: Embedder, store: Store) {
		this.#ai = ai;
		this.#store = store;
	}

	async query(q: string): Promise<Array<Result>> {
		const [vector] = await this.#embed([`task: search result | query: ${q}`]);
		// Pages can have several chunks, so ask for more matches and keep the best one per page.
		const { matches } = await this.#store.query(vector!, { returnMetadata: 'all', topK: LIMIT * 2 });

		const results = new Map<string, Result>();

		for (const { metadata, score } of matches) {
			const { title, url } = metadata as Meta;

			if (!results.has(url)) {
				results.set(url, { score, title, url } satisfies Result);
			}
		}

		return [...results.values()].slice(0, LIMIT);
	}

	async index(chunks: Array<Chunk> = documents()): Promise<{ pages: number; vectors: number }> {
		for (let start = 0; start < chunks.length; start += BATCH) {
			const batch = chunks.slice(start, start + BATCH);

			const vectors = await this.#embed(batch.map(({ text }) => text));

			await this.#store.upsert(batch.map(({ id, title, url }, n) => ({ id, metadata: { title, url } satisfies Meta, values: vectors[n]! })));
		}

		return { pages: new Set(chunks.map(({ url }) => url)).size, vectors: chunks.length };
	}

	async #embed(input: Array<string>): Promise<Array<Array<number>>> {
		// The model returns 768 dimensions by default, matching the hara-docs index.
		const { data } = (await this.#ai.run(MODEL, { input })) as Embeddings;

		return data.map(({ embedding }) => embedding);
	}
}

async function digest(text: string): Promise<Uint8Array> {
	return new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(text)));
}

// Compares SHA-256 digests byte by byte, so the check takes the same time for every wrong token.
async function authorised(request: Request, token: string | undefined): Promise<boolean> {
	if (!token) {
		return false;
	}

	const [given, expected] = await Promise.all([digest(request.headers.get('authorization') ?? ''), digest(`Bearer ${token}`)]);

	return given.reduce((diff, byte, n) => diff | (byte ^ expected[n]!), 0) === 0;
}

async function readQueryBody(request: Request): Promise<string | null> {
	const reader = request.body?.getReader();

	if (!reader) {
		return '';
	}

	const bytes = new Uint8Array(MAX_QUERY_BODY);

	let length = 0;

	try {
		for (;;) {
			const chunk = await reader.read();

			if (chunk.done) {
				return new TextDecoder().decode(bytes.subarray(0, length));
			}

			if (chunk.value.byteLength > bytes.length - length) {
				await reader.cancel();

				return null;
			}

			bytes.set(chunk.value, length);
			length += chunk.value.byteLength;
		}
	} finally {
		reader.releaseLock();
	}
}

function question(text: string): Nullable<string> {
	let body: unknown;

	try {
		body = JSON.parse(text);
	} catch {
		return null;
	}

	const q = (body as Nullable<{ q?: unknown }>)?.q;

	if (typeof q !== 'string' || !q.trim() || q.length > MAX_QUERY) {
		return null;
	}

	return q.trim();
}

function rateLimited(): Response {
	return Response.json({ error: 'Too many search requests.' }, { headers: { 'Retry-After': '60' }, status: 429 });
}

async function query(request: Request, env: SearchEnv, search: Search): Promise<Response> {
	const client = request.headers.get('CF-Connecting-IP') ?? 'unknown';

	if (!(await env.SEARCH_LIMIT.limit({ key: client })).success) {
		return rateLimited();
	}

	const body = await readQueryBody(request);

	if (body === null) {
		return Response.json({ error: 'Search request is too large.' }, { status: 413 });
	}

	const q = question(body);

	if (q === null) {
		return Response.json({ error: `Send JSON with a q of 1 to ${MAX_QUERY} characters.` }, { status: 400 });
	}

	const hash = Array.from(await digest(`${MODEL}\n${q}`), (byte) => byte.toString(16).padStart(2, '0')).join('');

	const key = new URL(`/api/search/cache/${hash}`, request.url).toString();

	const cache = await caches.open('hara-docs-search');

	const hit = await cache.match(key);

	if (hit) {
		return hit;
	}

	// The shared budget also limits cache misses from many different clients in one location.
	if (!(await env.SEARCH_BUDGET.limit({ key: 'query' })).success) {
		return rateLimited();
	}

	const response = Response.json(await search.query(q), { headers: { 'Cache-Control': 'max-age=300' } });

	await cache.put(key, response.clone());

	return response;
}

export async function api(request: Request, env: SearchEnv, path: string): Promise<Response> {
	if (path !== '/api/search/query' && path !== '/api/search/index') {
		return Response.json({ error: 'Not found.' }, { status: 404 });
	}

	if (request.method !== 'POST') {
		return Response.json({ error: 'Use POST.' }, { headers: { allow: 'POST' }, status: 405 });
	}

	const search = new Search(env.AI, env.VECTORIZE);

	try {
		if (path === '/api/search/index') {
			if (!(await authorised(request, env.INDEX_TOKEN))) {
				return Response.json({ error: 'Unauthorised.' }, { status: 401 });
			}

			return Response.json(await search.index());
		}

		return await query(request, env, search);
	} catch {
		return Response.json({ error: 'Search is unavailable.' }, { status: 502 });
	}
}
