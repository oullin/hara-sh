import { Effect } from 'effect';
import { describe, expect, it } from 'vitest';
import { InvalidJson, ModelNotAllowed } from '@/codex/errors';
import { BodyReadError, CodexRejected } from '@/codex/program';
import { runProgram } from '@/effect/run';
import { Upstream, UpstreamError } from '@/upstream';
import { fakeUpstream } from '@test/helpers';

const { service } = fakeUpstream(new Response('from upstream'));

describe('runProgram', () => {
	it('provides the Upstream service and returns the response', async () => {
		const res = await runProgram(
			Upstream.use((upstream) => upstream.forward(new Request('https://proxy.test/'))),
			service,
		);

		expect(await res.text()).toBe('from upstream');
	});

	it.each([
		['BodyReadError', new BodyReadError({ cause: 'x' }), 400, 'invalid_request_error'],
		['InvalidJson', new CodexRejected({ reason: new InvalidJson({ message: 'invalid JSON body', status: 400 }) }), 400, 'invalid_request_error'],
		['ModelNotAllowed', new CodexRejected({ reason: new ModelNotAllowed({ message: 'no', model: 'claude', status: 403 }) }), 403, 'permission_error'],
		['UpstreamError', new UpstreamError({ cause: 'down' }), 502, 'upstream_error'],
	] as const)('maps %s to an HTTP error', async (_, error, status, type) => {
		const res = await runProgram(
			Effect.fail(error),
			service,
		);

		expect(res.status).toBe(status);

		expect(await res.json()).toMatchObject({ error: { type } });
	});
});
