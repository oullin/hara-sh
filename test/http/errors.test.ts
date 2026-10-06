import { describe, expect, it } from 'vitest';
import { deny } from '@/http/errors';

describe('deny', () => {
	it('returns a JSON permission error with the given status', async () => {
		const res = deny(403, 'nope');

		expect(res.status).toBe(403);
		expect(
			res.headers.get('content-type'),
		).toContain('application/json');

		expect(await res.json()).toEqual({ error: { message: 'nope', type: 'permission_error' } });
	});
});
