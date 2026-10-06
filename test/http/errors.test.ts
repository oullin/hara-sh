import { describe, expect, it } from 'vitest';
import { deny, errorResponse } from '@/http/errors';

describe('errorResponse', () => {
	it('returns a JSON error with the given status and type', async () => {
		const res = errorResponse(502, 'upstream_error', 'down');

		expect(res.status).toBe(502);
		expect(
			res.headers.get('content-type'),
		).toContain('application/json');

		expect(await res.json()).toEqual({ error: { message: 'down', type: 'upstream_error' } });
	});
});

describe('deny', () => {
	it('returns a permission error', async () => {
		const res = deny(403, 'nope');

		expect(res.status).toBe(403);

		expect(await res.json()).toEqual({ error: { message: 'nope', type: 'permission_error' } });
	});
});
