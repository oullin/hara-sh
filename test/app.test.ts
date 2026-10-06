import { beforeEach, describe, expect, it, vi } from 'vitest';
import { app } from '@/app';
import { fakeContext, post } from '@test/helpers';

const env = { CODEX_API_KEY: 'codex-key' } as Env;
const codex = { authorization: 'Bearer codex-key' };

describe('app: main key', () => {
	it('forwards requests untouched to the container', async () => {
		const { ctx, requests } = fakeContext();

		const request = post(
			'/v1/messages',
			{ model: 'claude-opus-4-8' },
			{ 'x-api-key': 'claude-key' },
		);

		const res = await app.fetch(request, env, ctx);

		expect(await res.text()).toBe('from container');

		expect(requests).toHaveLength(1);
	});

	it('forwards everything when no Codex key is configured', async () => {
		const { ctx, requests } = fakeContext();

		await app.fetch(post('/v1/messages', {}, codex), {} as Env, ctx);

		expect(requests).toHaveLength(1);
	});

	it('answers 502 when the container is unreachable', async () => {
		const { ctx } = fakeContext(() => Promise.reject(new Error('down')));

		const res = await app.fetch(new Request('https://proxy.test/healthz'), env, ctx);

		expect(res.status).toBe(502);
	});
});

describe('app: Codex key', () => {
	beforeEach(() => {
		vi.spyOn(console, 'log').mockImplementation(() => {});
	});

	it.each(['/v1/responses', '/v1/chat/completions'])('forwards %s for Codex models with Fast mode', async (path) => {
		const { ctx, requests } = fakeContext();

		const res = await app.fetch(post(path, { model: 'gpt-6.1-sol' }, codex), env, ctx);

		expect(res.status).toBe(200);

		expect(await requests[0].json()).toEqual({ model: 'gpt-6.1-sol', service_tier: 'priority' });
	});

	it('refuses Claude models', async () => {
		const { ctx, requests } = fakeContext();

		const res = await app.fetch(post('/v1/responses', { model: 'claude-opus-4-8' }, codex), env, ctx);

		expect(res.status).toBe(403);
		expect(requests).toHaveLength(0);
	});

	it('refuses invalid JSON', async () => {
		const { ctx } = fakeContext();

		const res = await app.fetch(post('/v1/responses', '{nope', codex), env, ctx);

		expect(res.status).toBe(400);
	});

	it('filters the model catalog', async () => {
		const { ctx } = fakeContext(() => Promise.resolve(Response.json({ models: [{ slug: 'gpt-5.5' }, { slug: 'claude-sonnet-5' }] })));

		const res = await app.fetch(new Request('https://proxy.test/v1/models', { headers: codex }), env, ctx);

		expect(await res.json()).toEqual({ models: [{ slug: 'gpt-5.5' }] });
	});

	it('refuses other paths with 403', async () => {
		const { ctx, requests } = fakeContext();

		const res = await app.fetch(post('/v1/messages', { model: 'gpt-5.5' }, codex), env, ctx);

		expect(res.status).toBe(403);

		expect(await res.json()).toMatchObject({ error: { message: 'codex key cannot access /v1/messages' } });

		expect(requests).toHaveLength(0);
	});

	it('refuses the wrong method on a Codex path with 405', async () => {
		const { ctx } = fakeContext();

		const res = await app.fetch(new Request('https://proxy.test/v1/responses', { headers: codex }), env, ctx);

		expect(res.status).toBe(405);
	});

	it('refuses websocket upgrades', async () => {
		const { ctx, requests } = fakeContext();

		const res = await app.fetch(new Request('https://proxy.test/v1/responses', { headers: { ...codex, upgrade: 'websocket' } }), env, ctx);

		expect(res.status).toBe(403);
		expect(requests).toHaveLength(0);
	});
});
