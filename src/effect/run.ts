import { Effect } from 'effect';
import { matchError } from 'better-result';
import type { CodexProgramError } from '@/codex/program';
import { errorResponse } from '@/http/errors';
import { Upstream } from '@/upstream';

// Bridge from Hono to Effect: provide the per-request Upstream service, turn every typed
// failure into an HTTP response, and run it as a Promise.
export function runProgram(program: Effect.Effect<Response, CodexProgramError, Upstream>, upstream: Upstream['Service']): Promise<Response> {
	return program.pipe(
		Effect.catchTags({
			BodyReadError: () => Effect.succeed(errorResponse(400, 'invalid_request_error', 'could not read the request or upstream body')),
			CodexRejected: ({ reason }) =>
				Effect.succeed(
					matchError(reason, {
						InvalidJson: (e) => errorResponse(e.status, 'invalid_request_error', e.message),
						ModelNotAllowed: (e) => errorResponse(e.status, 'permission_error', e.message),
					}),
				),
			UpstreamError: () => Effect.succeed(errorResponse(502, 'upstream_error', 'the proxy container is unavailable')),
		}),
		Effect.provideService(Upstream, upstream),
		Effect.runPromise,
	);
}
