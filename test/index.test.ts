import { describe, expect, it, vi } from 'vitest';

vi.mock('@cloudflare/containers', () => ({ Container: class {} }));

const { default: worker } = await import('@/index');

function setup() {
	const stub = { fetch: vi.fn(() => Promise.resolve(new Response('from container'))) };

	const ctx = {
		exports: { CliProxy: { get: () => stub, idFromName: (name: string) => name } },
	} as unknown as ExecutionContext;

	return { ctx, stub };
}

const fetchWorker = (request: Request, env: Partial<Env>, ctx: ExecutionContext) => worker.fetch(request as Parameters<typeof worker.fetch>[0], env as Env, ctx);

describe('worker fetch', () => {
	it('sends main-key requests straight to the container', async () => {
		const { ctx, stub } = setup();
		const request = new Request('https://proxy.test/v1/messages', { headers: { 'x-api-key': 'claude-key' } });

		const res = await fetchWorker(
			request,
			{ CODEX_API_KEY: 'codex-key' },
			ctx,
		);

		expect(await res.text()).toBe('from container');

		expect(stub.fetch).toHaveBeenCalledWith(request);
	});

	it('sends Codex-key requests through the Codex guard', async () => {
		const { ctx, stub } = setup();
		const request = new Request('https://proxy.test/v1/messages', { headers: { authorization: 'Bearer codex-key' } });

		const res = await fetchWorker(
			request,
			{ CODEX_API_KEY: 'codex-key' },
			ctx,
		);

		expect(res.status).toBe(403);
		expect(stub.fetch).not.toHaveBeenCalled();
	});

	it('skips the guard when no Codex key is configured', async () => {
		const { ctx, stub } = setup();
		const request = new Request('https://proxy.test/v1/messages', { headers: { authorization: 'Bearer anything' } });

		await fetchWorker(
			request,
			{},
			ctx,
		);

		expect(stub.fetch).toHaveBeenCalledWith(request);
	});

	it('re-exports the Durable Object class', async () => {
		const mod = await import('@/index');

		expect(mod.CliProxy).toBeTypeOf('function');
	});
});
