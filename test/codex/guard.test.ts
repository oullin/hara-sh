import { beforeEach, describe, expect, it, vi } from 'vitest';
import { codexOnly } from '@/codex/guard';
import { fakeUpstream, post } from '@test/helpers';

const models = (body: unknown, init: ResponseInit = {}) => new Response(JSON.stringify(body), init);

describe('codexOnly', () => {
	let log: ReturnType<typeof vi.spyOn>;

	beforeEach(() => {
		log = vi.spyOn(console, 'log').mockImplementation(() => {});
	});

	describe('path and transport checks', () => {
		it('blocks paths outside the Codex endpoints', async () => {
			const { upstream } = fakeUpstream();

			const res = await codexOnly(
				post(
					'/v1/messages',
					{ model: 'gpt-5.5' },
				),
				upstream,
			);

			expect(res.status).toBe(403);

			expect(await res.json()).toMatchObject({ error: { message: 'codex key cannot access /v1/messages' } });

			expect(upstream).not.toHaveBeenCalled();
		});

		it('blocks websocket upgrades', async () => {
			const { upstream } = fakeUpstream();
			const request = new Request('https://proxy.test/v1/responses', { headers: { upgrade: 'WebSocket' } });

			const res = await codexOnly(request, upstream);

			expect(res.status).toBe(403);
			expect(upstream).not.toHaveBeenCalled();
		});

		it('rejects non-POST requests to generation endpoints', async () => {
			const { upstream } = fakeUpstream();

			const res = await codexOnly(new Request('https://proxy.test/v1/responses'), upstream);

			expect(res.status).toBe(405);
			expect(upstream).not.toHaveBeenCalled();
		});
	});

	describe('request bodies', () => {
		it('rejects invalid JSON', async () => {
			const { upstream } = fakeUpstream();

			const res = await codexOnly(
				post('/v1/responses', '{not json'),
				upstream,
			);

			expect(res.status).toBe(400);
			expect(upstream).not.toHaveBeenCalled();
		});

		it('rejects non-Codex models', async () => {
			const { upstream } = fakeUpstream();

			const res = await codexOnly(
				post(
					'/v1/responses',
					{ model: 'claude-opus-4-8' },
				),
				upstream,
			);

			expect(res.status).toBe(403);

			expect(await res.json()).toMatchObject({
				error: { message: 'codex key is limited to Codex models; "claude-opus-4-8" is not allowed' },
			});

			expect(upstream).not.toHaveBeenCalled();
		});

		it('rejects requests without a model', async () => {
			const { upstream } = fakeUpstream();

			const res = await codexOnly(
				post(
					'/v1/chat/completions',
					{ input: 'hi' },
				),
				upstream,
			);

			expect(res.status).toBe(403);
			expect(upstream).not.toHaveBeenCalled();
		});

		it('adds the priority tier when none is requested and logs it as a default', async () => {
			const { calls, upstream } = fakeUpstream(new Response('done'));

			const res = await codexOnly(
				post(
					'/v1/responses',
					{ input: 'hi', model: 'gpt-6.1-sol', reasoning: { effort: 'high' } },
				),
				upstream,
			);

			expect(await res.text()).toBe('done');

			expect(calls).toHaveLength(1);
			expect(calls[0].method).toBe('POST');

			expect(await calls[0].json()).toEqual({ input: 'hi', model: 'gpt-6.1-sol', reasoning: { effort: 'high' }, service_tier: 'priority' });

			expect(log).toHaveBeenCalledWith('codex request model=gpt-6.1-sol tier=priority (default) effort=high');
		});

		it('forwards an explicit tier untouched', async () => {
			const { calls, upstream } = fakeUpstream();
			const raw = JSON.stringify({ model: 'gpt-5.5', service_tier: 'default' });

			await codexOnly(
				post('/v1/responses', raw),
				upstream,
			);

			expect(await calls[0].text()).toBe(raw);

			expect(log).toHaveBeenCalledWith('codex request model=gpt-5.5 tier=default effort=unset');
		});
	});

	describe('/v1/models', () => {
		it('filters the OpenAI list and the Codex catalog to Codex models', async () => {
			const { upstream } = fakeUpstream(
				models(
					{
						data: [{ id: 'gpt-5.5' }, { id: 'claude-opus-4-8' }],
						models: [{ slug: 'gpt-6.1-sol' }, { slug: 'claude-sonnet-5' }, { id: 'codex-auto-review' }],
						object: 'list',
					},
					{ headers: { 'content-encoding': 'identity', 'content-length': '999', etag: '"abc"', 'x-extra': 'kept' }, status: 200 },
				),
			);

			const res = await codexOnly(new Request('https://proxy.test/v1/models?client_version=0.160.0'), upstream);

			expect(await res.json()).toEqual({
				data: [{ id: 'gpt-5.5' }],
				models: [{ slug: 'gpt-6.1-sol' }, { id: 'codex-auto-review' }],
				object: 'list',
			});

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

		it('leaves payloads without model lists untouched', async () => {
			const { upstream } = fakeUpstream(
				models(
					{ data: 'not-a-list', object: 'list' },
				),
			);

			const res = await codexOnly(new Request('https://proxy.test/v1/models'), upstream);

			expect(await res.json()).toEqual({ data: 'not-a-list', object: 'list' });
		});

		it('passes upstream errors through unchanged', async () => {
			const { upstream } = fakeUpstream(new Response('Invalid API key', { status: 401 }));

			const res = await codexOnly(new Request('https://proxy.test/v1/models'), upstream);

			expect(res.status).toBe(401);

			expect(await res.text()).toBe('Invalid API key');
		});
	});
});
