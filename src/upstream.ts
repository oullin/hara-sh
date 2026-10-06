import { Context, Data, Effect } from 'effect';
import type { CliProxy } from '@/container/cli-proxy';

// The container (or the Durable Object in front of it) could not be reached.
export class UpstreamError extends Data.TaggedError('UpstreamError')<{ cause: unknown }> {}

// Anthropic's OAuth endpoint rejects the default placement (near Dubai) with 403
// "Request not allowed", so the Durable Object (and its container) is created in Western
// Europe. Location hints only apply when a DO is first created, hence the region in the name.
const INSTANCE_REGION: DurableObjectLocationHint = 'weur';

// Side effect boundary: every call to the CLIProxyAPI container goes through this service.
export class Upstream extends Context.Service<Upstream, { forward(request: Request): Effect.Effect<Response, UpstreamError> }>()('cli-proxy-api/Upstream') {}

// One named instance so every request shares the same auth state.
export function cliProxyUpstream(ctx: ExecutionContext): Upstream['Service'] {
	// ctx.exports.CliProxy is the loopback namespace for the sqlite-backed DO declared in
	// cloudflare.config.ts; its generated type is not narrowed yet.
	const namespace = ctx.exports.CliProxy as unknown as DurableObjectNamespace<CliProxy>;
	const stub = namespace.get(namespace.idFromName(`main-${INSTANCE_REGION}`), { locationHint: INSTANCE_REGION });

	return Upstream.of({
		forward: (request) =>
			Effect.tryPromise({
				catch: (cause) => new UpstreamError({ cause }),
				try: () => stub.fetch(request),
			}),
	});
}
