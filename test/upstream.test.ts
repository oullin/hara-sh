import { describe, expect, it, vi } from 'vitest';
import { cliProxyUpstream } from '@/upstream';

describe('cliProxyUpstream', () => {
	it('routes every request to the single Western Europe instance', async () => {
		const stub = { fetch: vi.fn(() => Promise.resolve(new Response('from container'))) };

		const namespace = {
			get: vi.fn(() => stub),
			idFromName: vi.fn((name: string) => ({ name })),
		};

		const ctx = { exports: { CliProxy: namespace } } as unknown as ExecutionContext;
		const upstream = cliProxyUpstream(ctx);
		const request = new Request('https://proxy.test/healthz');

		const res = await upstream(request);

		expect(namespace.idFromName).toHaveBeenCalledWith('main-weur');
		expect(namespace.get).toHaveBeenCalledWith({ name: 'main-weur' }, { locationHint: 'weur' });
		expect(stub.fetch).toHaveBeenCalledWith(request);

		expect(await res.text()).toBe('from container');
	});
});
