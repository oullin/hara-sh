import type { Nullable, Result } from '../../../worker/types.ts';

export type Fetcher = (input: string, init: RequestInit) => Promise<Response>;

export class SemanticSearch {
	readonly #fetch: Fetcher;

	constructor(fetcher: Fetcher = (input, init) => fetch(input, init)) {
		this.#fetch = fetcher;
	}

	// Null means the API failed or is missing (as under `vitepress dev`), so the caller falls back to local search.
	async query(q: string): Promise<Nullable<Array<Result>>> {
		try {
			const response = await this.#fetch('/api/search/query', {
				body: JSON.stringify({ q }),
				headers: { 'content-type': 'application/json' },
				method: 'POST',
			});

			return response.ok ? ((await response.json()) as Array<Result>) : null;
		} catch {
			return null;
		}
	}
}
