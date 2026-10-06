import { describe, expect, it } from 'vitest';
import { authorizeCodexBody, filterCodexModels } from '@/codex/authorize';
import { InvalidJson, ModelNotAllowed } from '@/codex/errors';

describe('authorizeCodexBody', () => {
	it('fails with InvalidJson for a malformed body', () => {
		const result = authorizeCodexBody('{nope');

		expect(result.isErr() && InvalidJson.is(result.error)).toBe(true);
		expect(result.isErr() && result.error.status).toBe(400);
	});

	it('fails with ModelNotAllowed for a non-Codex model', () => {
		const result = authorizeCodexBody(
			JSON.stringify({ model: 'claude-opus-4-8' }),
		);

		expect(result.isErr() && ModelNotAllowed.is(result.error)).toBe(true);
		expect(result.isErr() && result.error.message).toBe('codex key is limited to Codex models; "claude-opus-4-8" is not allowed');
	});

	it('fails with ModelNotAllowed when the model is missing', () => {
		const result = authorizeCodexBody(
			JSON.stringify({ input: 'hi' }),
		);

		expect(result.isErr() && result.error._tag).toBe('ModelNotAllowed');
	});

	it('adds the priority tier when none is requested', () => {
		const result = authorizeCodexBody(
			JSON.stringify({ input: 'hi', model: 'gpt-6.1-sol', reasoning: { effort: 'high' } }),
		);

		expect(
			result.unwrap(),
		).toEqual({
			body: JSON.stringify({ input: 'hi', model: 'gpt-6.1-sol', reasoning: { effort: 'high' }, service_tier: 'priority' }),
			effort: 'high',
			model: 'gpt-6.1-sol',
			tier: 'priority (default)',
		});
	});

	it('forwards an explicit tier byte-for-byte', () => {
		const raw = JSON.stringify({ model: 'gpt-5.5', service_tier: 'default' });

		expect(
			authorizeCodexBody(raw).unwrap(),
		).toEqual({ body: raw, effort: 'unset', model: 'gpt-5.5', tier: 'default' });
	});
});

describe('filterCodexModels', () => {
	it('filters both the OpenAI list and the Codex catalog', () => {
		const body = {
			data: [{ id: 'gpt-5.5' }, { id: 'claude-opus-4-8' }],
			models: [{ slug: 'gpt-6.1-sol' }, { slug: 'claude-sonnet-5' }, { id: 'codex-auto-review' }],
			object: 'list',
		};

		expect(
			filterCodexModels(body),
		).toEqual({
			data: [{ id: 'gpt-5.5' }],
			models: [{ slug: 'gpt-6.1-sol' }, { id: 'codex-auto-review' }],
			object: 'list',
		});
		expect(body.data).toHaveLength(2);
	});

	it('leaves non-list fields untouched', () => {
		expect(
			filterCodexModels(
				{ data: 'not-a-list', object: 'list' },
			),
		).toEqual({ data: 'not-a-list', object: 'list' });
	});
});
