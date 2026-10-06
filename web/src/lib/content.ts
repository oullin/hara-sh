export type Provider = { glyph: string; name: string; via: string };

// 5×5 glyphs are abstract marks, not provider logos.
export const providers: Array<Provider> = [
	{ glyph: '..#..|#.#.#|.###.|#.#.#|..#..', name: 'Claude', via: 'OAuth · API key' },
	{ glyph: '.#.#.|#...#|.....|#...#|.#.#.', name: 'Codex', via: 'OAuth · device code' },
	{ glyph: '..#..|.###.|#####|.###.|..#..', name: 'Gemini', via: 'API key · Vertex' },
	{ glyph: '..#..|.#.#.|#...#|.....|#...#', name: 'Antigravity', via: 'OAuth' },
	{ glyph: '#...#|.#.#.|..#..|.#.#.|#...#', name: 'Grok', via: 'OAuth · API key' },
	{ glyph: '#..#.|#.#..|##...|#.#..|#..#.', name: 'Kimi', via: 'OAuth' },
	{ glyph: '###..|#..#.|#...#|#..#.|###..', name: 'Devin', via: 'OAuth' },
	{ glyph: '.....|.#.#.|#.#.#|#...#|.....', name: 'Meta', via: 'OAuth · API key' },
	{ glyph: '#####|#...#|#.#.#|#...#|#####', name: 'OpenAI-compatible', via: 'Any endpoint + key' },
	{ glyph: '..#..|..#..|#####|..#..|..#..', name: 'Plugins', via: 'Copilot, Kiro and more' },
];

export const clients = ['Claude Code', 'Codex CLI and app', 'Any OpenAI client'];

export const guarantees = ['Same account per conversation', 'Failover at the limit', 'One key per client, scoped'];

export type Feature = { body: string; title: string };

export const features: Array<Feature> = [
	{ body: 'A conversation and its subagents stay on the account that started it, so cached prompts are reused.', title: 'Caches stay warm' },
	{ body: 'When one account hits its window, the next request moves to another without a setting changing.', title: 'Limits are not your problem' },
	{ body: "Give a client a key that only reaches one provider's models. It is checked before the request leaves the edge.", title: 'Keys with a scope' },
];

export const PROXY_URL = 'https://proxy.hara.sh';

export const SETUP = {
	claude: `export ANTHROPIC_BASE_URL=${PROXY_URL}\nexport ANTHROPIC_AUTH_TOKEN=[YOUR KEY]`,
	codex: `model_provider = "hara"\n[model_providers.hara]\nbase_url = "${PROXY_URL}/v1"`,
	hero: `export ANTHROPIC_BASE_URL=${PROXY_URL}`,
};
