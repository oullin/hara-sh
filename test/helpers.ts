import { vi } from 'vitest';
import type { Upstream } from '@/upstream';

// An upstream stub that records forwarded requests and answers with `response`.
export function fakeUpstream(response: Response = new Response('ok')) {
	const calls: Array<Request> = [];

	const upstream = vi.fn<Upstream>((request) => {
		calls.push(request);

		return Promise.resolve(response.clone());
	});

	return { calls, upstream };
}

export function post(path: string, body: unknown, headers: Record<string, string> = {}): Request {
	return new Request(`https://proxy.test${path}`, {
		body: typeof body === 'string' ? body : JSON.stringify(body),
		headers: { 'content-type': 'application/json', ...headers },
		method: 'POST',
	});
}
