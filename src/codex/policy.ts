// What the Codex client key (CODEX_API_KEY) may reach. The proxy serves these models only
// from the Codex OAuth accounts, so the key can never draw from the Claude accounts.
const CODEX_MODEL = /^(gpt-|codex-|o\d)/i;

export const CODEX_PATHS = new Set(["/v1/responses", "/v1/chat/completions", "/v1/models"]);

export function isCodexModel(model: unknown): model is string {
  return typeof model === "string" && CODEX_MODEL.test(model);
}

// Fast mode ("2x speed, increased usage"). The Codex app hides its Fast toggle and sends no
// service_tier for custom providers, so Codex-key requests without a tier get this one.
// An explicit tier from the client (e.g. "default") is left untouched.
export const DEFAULT_SERVICE_TIER = "priority";
