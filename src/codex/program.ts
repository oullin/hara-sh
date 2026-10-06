import { Console, Data, Effect } from 'effect';
import { authorizeCodexBody, filterCodexModels } from '@/codex/authorize';
import type { CodexDenied } from '@/codex/errors';
import { Upstream, type UpstreamError } from '@/upstream';

// The incoming request body or the upstream JSON could not be read.
export class BodyReadError extends Data.TaggedError('BodyReadError')<{ cause: unknown }> {}

export class CodexRejected extends Data.TaggedError('CodexRejected')<{ reason: CodexDenied }> {}

export type CodexProgramError = BodyReadError | CodexRejected | UpstreamError;

// Read the body, apply the Codex policy (better-result), log the audit line and forward.
export const forwardCodexRequest = Effect.fnUntraced(function* (request: Request) {
	const text = yield* Effect.tryPromise({ catch: (cause) => new BodyReadError({ cause }), try: () => request.text() });
	const decision = authorizeCodexBody(text);

	if (decision.isErr()) {
		return yield* new CodexRejected({ reason: decision.error });
	}

	const call = decision.value;

	// The upstream response always reports service_tier "default", so this line is the only
	// way to confirm Fast mode (`make tail-codex`). Metadata only; no prompt content.
	yield* Console.log(`codex request model=${call.model} tier=${call.tier} effort=${call.effort}`);

	const upstream = yield* Upstream;

	// Only POST routes reach this program; state it so the body is valid for the method.
	return yield* upstream.forward(new Request(request, { body: call.body, method: 'POST' }));
});

// Forward /v1/models and keep only Codex models. Upstream errors pass through unchanged.
export const listCodexModels = Effect.fnUntraced(function* (request: Request) {
	const upstream = yield* Upstream;
	const res = yield* upstream.forward(request);

	if (!res.ok) {
		return res;
	}

	const body = yield* Effect.tryPromise({ catch: (cause) => new BodyReadError({ cause }), try: () => res.json() as Promise<Record<string, unknown>> });
	// The body changed, so drop headers that describe the upstream bytes. Dropping the ETag also
	// stops a client from revalidating a previously cached, unfiltered catalog with a 304.
	const headers = new Headers(res.headers);

	for (const name of ['content-length', 'content-encoding', 'etag']) {
		headers.delete(name);
	}

	return Response.json(filterCodexModels(body), { headers, status: res.status });
});
