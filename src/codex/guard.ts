import { deny } from '@/http/errors';
import type { Upstream } from '@/upstream';
import { CODEX_PATHS, DEFAULT_SERVICE_TIER, isCodexModel } from '@/codex/policy';

// Forwards a request made with the Codex key only if it targets a Codex model.
export async function codexOnly(request: Request, upstream: Upstream): Promise<Response> {
	const { pathname } = new URL(request.url);

	if (!CODEX_PATHS.has(pathname)) {
		return deny(403, `codex key cannot access ${pathname}`);
	}
	// Model names inside a WebSocket stream cannot be checked here; Codex falls back to HTTP.
	if (request.headers.get('upgrade')?.toLowerCase() === 'websocket') {
		return deny(403, 'codex key cannot use websockets');
	}

	if (pathname === '/v1/models') {
		return listCodexModels(request, upstream);
	}

	if (request.method !== 'POST') {
		return deny(405, 'method not allowed');
	}

	const text = await request.text();

	let payload: { model?: unknown; reasoning?: { effort?: unknown }; service_tier?: unknown };

	try {
		payload = JSON.parse(text);
	} catch {
		return deny(400, 'invalid JSON body');
	}

	if (!isCodexModel(payload.model)) {
		return deny(403, `codex key is limited to Codex models; "${String(payload.model)}" is not allowed`);
	}

	const requestedTier = payload.service_tier;
	const body = requestedTier === undefined ? JSON.stringify({ ...payload, service_tier: DEFAULT_SERVICE_TIER }) : text;

	// The upstream response always reports service_tier "default", so this line is the only way
	// to confirm Fast mode (`make logs-cf`). Metadata only; no prompt content.
	// oxlint-disable-next-line no-console -- intentional: the request audit line read by `make tail-codex`
	console.log(`codex request model=${payload.model} tier=${String(requestedTier ?? `${DEFAULT_SERVICE_TIER} (default)`)} effort=${String(payload.reasoning?.effort ?? 'unset')}`);
	// Only POST reaches this point (checked above); state it so the body is valid for the method.
	return upstream(new Request(request, { body, method: 'POST' }));
}

type ModelEntry = { id?: string; slug?: string };

// /v1/models comes in two shapes: the OpenAI list ({ data: [{ id }] }) and, when Codex sends
// ?client_version=..., the Codex catalog ({ models: [{ slug }] }) that drives its model picker.
// Filter whichever lists are present and leave the rest of the payload untouched.
async function listCodexModels(request: Request, upstream: Upstream): Promise<Response> {
	const res = await upstream(request);

	if (!res.ok) {
		return res;
	}

	const body = (await res.json()) as Record<string, unknown>;

	for (const key of ['data', 'models']) {
		const list = body[key];

		if (Array.isArray(list)) {
			body[key] = (list as Array<ModelEntry>).filter((m) => isCodexModel(m.slug ?? m.id));
		}
	}
	// The body changed, so drop headers that describe the upstream bytes. Dropping the ETag also
	// stops a client from revalidating a previously cached, unfiltered catalog with a 304.
	const headers = new Headers(res.headers);

	for (const name of ['content-length', 'content-encoding', 'etag']) {
		headers.delete(name);
	}

	return Response.json(body, { headers, status: res.status });
}
