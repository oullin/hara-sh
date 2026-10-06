import { describe, expect, it } from 'vitest';
import { clientKey, sameKey } from '@/auth/client-key';

const req = (headers: Record<string, string>) => new Request('https://proxy.test/', { headers });

describe('clientKey', () => {
	it('reads a bearer token, case-insensitively and trimmed', () => {
		expect(
			clientKey(
				req(
					{ authorization: 'Bearer  abc ' },
				),
			),
		).toBe('abc');
		expect(
			clientKey(
				req(
					{ authorization: 'bearer xyz' },
				),
			),
		).toBe('xyz');
	});

	it('falls back to x-api-key when there is no bearer token', () => {
		expect(
			clientKey(
				req(
					{ 'x-api-key': 'k1' },
				),
			),
		).toBe('k1');
		expect(
			clientKey(
				req(
					{ authorization: 'Basic dXNlcg==', 'x-api-key': 'k2' },
				),
			),
		).toBe('k2');
	});

	it('returns an empty string when no key is sent', () => {
		expect(
			clientKey(
				req(
					{},
				),
			),
		).toBe('');
	});
});

describe('sameKey', () => {
	it('matches identical keys', () => {
		expect(
			sameKey('secret', 'secret'),
		).toBe(true);
	});

	it('rejects keys of a different length', () => {
		expect(
			sameKey('secret', 'secrets'),
		).toBe(false);
	});

	it('rejects different keys of the same length', () => {
		expect(
			sameKey('secret', 'secreT'),
		).toBe(false);
	});
});
