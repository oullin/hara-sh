import { Result } from 'better-result';
import { InvalidJson, ModelNotAllowed, type CodexDenied } from '@/codex/errors';
import { DEFAULT_SERVICE_TIER, isCodexModel } from '@/codex/policy';

type Payload = { model?: unknown; reasoning?: { effort?: unknown }; service_tier?: unknown };

// A Codex-key request that passed the policy, ready to forward.
export type CodexCall = {
	body: string;
	effort: string;
	model: string;
	tier: string;
};

const parse = (text: string): Result<Payload, InvalidJson> =>
	Result.try({
		catch: () => new InvalidJson({ message: 'invalid JSON body', status: 400 }),
		try: () => JSON.parse(text) as Payload,
	});

const requireCodexModel = (payload: Payload): Result<Payload & { model: string }, ModelNotAllowed> =>
	isCodexModel(payload.model)
		? Result.ok(payload as Payload & { model: string })
		: Result.err(
				new ModelNotAllowed({
					message: `codex key is limited to Codex models; "${String(payload.model)}" is not allowed`,
					model: payload.model,
					status: 403,
				}),
			);

// Pure policy decision for a Codex-key request body. Requests without a service_tier get
// Fast mode (DEFAULT_SERVICE_TIER); an explicit tier is forwarded byte-for-byte.
export function authorizeCodexBody(text: string): Result<CodexCall, CodexDenied> {
	return parse(text)
		.andThen(requireCodexModel)
		.map((payload) => {
			const explicit = payload.service_tier !== undefined;

			return {
				body: explicit ? text : JSON.stringify({ ...payload, service_tier: DEFAULT_SERVICE_TIER }),
				effort: String(payload.reasoning?.effort ?? 'unset'),
				model: payload.model,
				tier: explicit ? String(payload.service_tier) : `${DEFAULT_SERVICE_TIER} (default)`,
			};
		});
}

type ModelEntry = { id?: string; slug?: string };

// /v1/models comes in two shapes: the OpenAI list ({ data: [{ id }] }) and, when Codex sends
// ?client_version=..., the Codex catalog ({ models: [{ slug }] }) that drives its model picker.
// Keep only Codex models in whichever lists are present; leave the rest untouched.
export function filterCodexModels(body: Record<string, unknown>): Record<string, unknown> {
	const filtered = { ...body };

	for (const key of ['data', 'models']) {
		const list = filtered[key];

		if (Array.isArray(list)) {
			filtered[key] = (list as Array<ModelEntry>).filter((m) => isCodexModel(m.slug ?? m.id));
		}
	}

	return filtered;
}
