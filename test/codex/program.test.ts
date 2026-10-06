import { Effect, Exit } from 'effect';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { forwardCodexRequest, listCodexModels } from '@/codex/program';
import { Upstream } from '@/upstream';
import { fakeUpstream, post } from '@test/helpers';

const run = <A, E>(effect: Effect.Effect<A, E, Upstream>, service: Upstream['Service']) => Effect.runPromiseExit(effect.pipe(Effect.provideService(Upstream, service)));

const failureTag = (exit: Exit.Exit<unknown, unknown>) => (Exit.isFailure(exit) ? JSON.stringify(exit.cause) : 'success');

describe('forwardCodexRequest', () => {
	let log: ReturnType<typeof vi.spyOn>;

	beforeEach(() => {
		log = vi.spyOn(console, 'log').mockImplementation(() => {});
	});

	it('forwards an authorized request with the default tier and logs the audit line', async () => {
		const { calls, service } = fakeUpstream(new Response('done'));

		const exit = await run(
			forwardCodexRequest(
				post(
					'/v1/responses',
					{ model: 'gpt-6.1-sol' },
				),
			),
			service,
		);

		expect(Exit.isSuccess(exit) && (await exit.value.text())).toBe('done');

		expect(calls[0].method).toBe('POST');

		expect(await calls[0].json()).toEqual({ model: 'gpt-6.1-sol', service_tier: 'priority' });

		expect(log).toHaveBeenCalledWith('codex request model=gpt-6.1-sol tier=priority (default) effort=unset');
	});

	it('fails with CodexRejected when the policy refuses the body', async () => {
		const { calls, service } = fakeUpstream();

		const exit = await run(
			forwardCodexRequest(
				post(
					'/v1/responses',
					{ model: 'claude-opus-4-8' },
				),
			),
			service,
		);

		expect(
			failureTag(exit),
		).toContain('CodexRejected');
		expect(calls).toHaveLength(0);
	});

	it('fails with BodyReadError when the body cannot be read', async () => {
		const { service } = fakeUpstream();
		const broken = { text: () => Promise.reject(new Error('stream closed')) } as unknown as Request;

		const exit = await run(
			forwardCodexRequest(broken),
			service,
		);

		expect(
			failureTag(exit),
		).toContain('BodyReadError');
	});

	it('propagates UpstreamError', async () => {
		const { service } = fakeUpstream(new Error('container down'));

		const exit = await run(
			forwardCodexRequest(
				post(
					'/v1/responses',
					{ model: 'gpt-5.5' },
				),
			),
			service,
		);

		expect(
			failureTag(exit),
		).toContain('UpstreamError');
	});
});

describe('listCodexModels', () => {
	const models = new Request('https://proxy.test/v1/models?client_version=0.160.0');

	it('filters the catalog and drops headers that describe the upstream bytes', async () => {
		const upstreamResponse = new Response(JSON.stringify({ models: [{ slug: 'gpt-5.5' }, { slug: 'claude-opus-4-8' }] }), {
			headers: { 'content-encoding': 'identity', 'content-length': '999', etag: '"abc"', 'x-extra': 'kept' },
		});

		const { service } = fakeUpstream(upstreamResponse);

		const exit = await run(
			listCodexModels(models),
			service,
		);

		const res = Exit.isSuccess(exit) ? exit.value : new Response();

		expect(await res.json()).toEqual({ models: [{ slug: 'gpt-5.5' }] });

		expect(
			res.headers.get('etag'),
		).toBeNull();
		expect(
			res.headers.get('content-encoding'),
		).toBeNull();
		expect(
			res.headers.get('x-extra'),
		).toBe('kept');
	});

	it('passes upstream errors through unchanged', async () => {
		const { service } = fakeUpstream(new Response('Invalid API key', { status: 401 }));

		const exit = await run(
			listCodexModels(models),
			service,
		);

		expect(Exit.isSuccess(exit) && exit.value.status).toBe(401);
	});

	it('fails with BodyReadError when the upstream JSON is malformed', async () => {
		const { service } = fakeUpstream(new Response('not json'));

		const exit = await run(
			listCodexModels(models),
			service,
		);

		expect(
			failureTag(exit),
		).toContain('BodyReadError');
	});
});
