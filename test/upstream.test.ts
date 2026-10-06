import { Effect, Exit } from 'effect';
import { describe, expect, it, vi } from 'vitest';
import { cliProxyUpstream } from '@/upstream';

function setup(fetch: () => Promise<Response>) {
	const stub = { fetch: vi.fn(fetch) };
	const namespace = { get: vi.fn(() => stub), idFromName: vi.fn((name: string) => ({ name })) };
	const ctx = { exports: { CliProxy: namespace } } as unknown as ExecutionContext;

	return { namespace, stub, upstream: cliProxyUpstream(ctx) };
}

describe('cliProxyUpstream', () => {
	it('forwards to the single Western Europe instance', async () => {
		const { namespace, stub, upstream } = setup(() => Promise.resolve(new Response('from container')));
		const request = new Request('https://proxy.test/healthz');

		const res = await Effect.runPromise(upstream.forward(request));

		expect(namespace.idFromName).toHaveBeenCalledWith('main-weur');
		expect(namespace.get).toHaveBeenCalledWith({ name: 'main-weur' }, { locationHint: 'weur' });
		expect(stub.fetch).toHaveBeenCalledWith(request);

		expect(await res.text()).toBe('from container');
	});

	it('fails with UpstreamError when the container is unreachable', async () => {
		const { upstream } = setup(() => Promise.reject(new Error('container down')));

		const exit = await Effect.runPromiseExit(upstream.forward(new Request('https://proxy.test/')));

		expect(Exit.isFailure(exit) && JSON.stringify(exit.cause)).toContain('UpstreamError');
	});
});
