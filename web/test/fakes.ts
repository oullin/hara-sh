import type { Embedder, Store } from '@/../worker/search';

const DIMENSIONS = 768;

// Hashes each word into one of 768 slots, so texts that share words point the same way.
function embedding(text: string): Array<number> {
	const values = Array.from({ length: DIMENSIONS }, () => 0);

	for (const word of text.toLowerCase().match(/[a-z0-9]+/g) ?? []) {
		let hash = 0;

		for (const char of word) {
			hash = (hash * 31 + char.charCodeAt(0)) % DIMENSIONS;
		}

		values[hash]! += 1;
	}

	return values;
}

function cosine(a: Array<number>, b: Array<number>): number {
	const dot = a.reduce((sum, value, n) => sum + value * b[n]!, 0);

	return dot / (Math.hypot(...a) * Math.hypot(...b) || 1);
}

export class FakeAi implements Embedder {
	readonly inputs: Array<Array<string>> = [];

	failing = false;

	run(model: string, { input }: { input: Array<string> }): Promise<unknown> {
		if (this.failing) {
			return Promise.reject(new Error(`${model} is unavailable`));
		}

		this.inputs.push(input);

		return Promise.resolve({ data: input.map((text) => ({ embedding: embedding(text) })) });
	}
}

type Vector = Parameters<Store['upsert']>[0][number];

// Keeps vectors in memory and ranks them by cosine similarity, as the hara-docs index does.
export class FakeIndex implements Store {
	readonly vectors = new Map<string, Vector>();

	query(vector: Array<number>, { topK }: { topK: number }) {
		const matches = [...this.vectors.values()]
			.map(({ id, metadata, values }) => ({ id, metadata, score: cosine(vector, values) }))
			.sort((a, b) => b.score - a.score)
			.slice(0, topK);

		return Promise.resolve({ count: matches.length, matches });
	}

	upsert(vectors: Array<Vector>) {
		for (const vector of vectors) {
			this.vectors.set(vector.id, vector);
		}

		return Promise.resolve({ count: vectors.length, ids: vectors.map(({ id }) => id) });
	}
}

export function search() {
	return { AI: new FakeAi(), INDEX_TOKEN: 'test-token', VECTORIZE: new FakeIndex() };
}
