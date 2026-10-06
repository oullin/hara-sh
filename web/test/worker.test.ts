import { describe, expect, it, vi } from 'vitest';
import worker, { CANONICAL_HOST } from '@/../worker/index';

function env() {
	return { ASSETS: { fetch: vi.fn((request: Request) => Promise.resolve(new Response(`asset ${new URL(request.url).pathname}`))) } };
}

describe('landing page worker', () => {
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
});
