import { Effect } from 'effect';
import { UpstreamError, type Upstream } from '@/upstream';

// An Upstream service double that records forwarded requests. Pass a Response to answer with
// it, or an Error to fail with UpstreamError.
export function fakeUpstream(outcome: Response | Error = new Response('ok')) {
	const calls: Array<Request> = [];

	const service: Upstream['Service'] = {
		forward: (request) => {
			calls.push(request);

			return outcome instanceof Error ? Effect.fail(new UpstreamError({ cause: outcome })) : Effect.succeed(outcome.clone());
		},
	};

	return { calls, service };
}

// A fake ExecutionContext whose CliProxy namespace answers every request with `respond`.
export function fakeContext(respond: (request: Request) => Promise<Response> = () => Promise.resolve(new Response('from container'))) {
	const requests: Array<Request> = [];

	const ctx = {
		exports: {
			CliProxy: {
				get: () => ({
					fetch: (request: Request) => {
						requests.push(request);

						return respond(request);
					},
				}),
				idFromName: (name: string) => name,
			},
		},
		passThroughOnException: () => {},
		props: {},
		waitUntil: () => {},
	} as unknown as ExecutionContext;

	return { ctx, requests };
}

export function post(path: string, body: unknown, headers: Record<string, string> = {}): Request {
	return new Request(`https://proxy.test${path}`, {
		body: typeof body === 'string' ? body : JSON.stringify(body),
		headers: { 'content-type': 'application/json', ...headers },
		method: 'POST',
	});
}
