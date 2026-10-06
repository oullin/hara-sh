import { describe, expect, it } from 'vitest';
import { CODEX_PATHS, DEFAULT_SERVICE_TIER, isCodexModel } from '@/codex/policy';

describe('isCodexModel', () => {
	it.each(['gpt-6.1-sol', 'GPT-5.5', 'codex-auto-review', 'o3', 'o4-mini'])('accepts %s', (model) => {
		expect(
			isCodexModel(model),
		).toBe(true);
	});

	it.each(['claude-opus-4-8', 'gemini-3-pro', 'omni', '', undefined, null, 42])('rejects %s', (model) => {
		expect(
			isCodexModel(model),
		).toBe(false);
	});
});

describe('policy constants', () => {
	it('allows only the Codex endpoints', () => {
		expect(
			[...CODEX_PATHS],
		).toEqual(['/v1/responses', '/v1/chat/completions', '/v1/models']);
	});

	it('defaults to the priority (Fast) tier', () => {
		expect(DEFAULT_SERVICE_TIER).toBe('priority');
	});
});
