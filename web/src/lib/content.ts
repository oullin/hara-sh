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

// Served by portless on this computer and announced on the local network (mDNS).
export const PROXY_URL = 'https://hara.local';

export const DOCS_URL = 'https://github.com/router-for-me/CLIProxyAPI';

export type Client = { command: string; id: string; label: string };

// The hero's "For …" switch: the one line each client needs.
export const clients: Array<Client> = [
	{ command: `export ANTHROPIC_BASE_URL=${PROXY_URL}`, id: 'claude', label: 'For Claude Code' },
	{ command: `base_url = "${PROXY_URL}/v1"`, id: 'codex', label: 'For Codex' },
	{ command: `curl ${PROXY_URL}/v1/models`, id: 'openai', label: 'For any client' },
];

export type Step = {
	body: string;
	code: string;
	file: string;
	id: string;
	title: string;
	token: string;
};

// "How it works": each step's code is shown in the sticky panel while the step is in view.
export const steps: Array<Step> = [
	{
		body: 'Sign in to each subscription once, or add an API key. Tokens stay on your own computer and refresh on their own.',
		code: 'auths/\n  claude-personal.json\n  claude-work.json\n  codex-pro.json\n  codex-team.json\n  gemini.key',
		file: 'auths/',
		id: 'connect',
		title: 'Connect your',
		token: 'accounts',
	},
	{
		body: 'Point Claude Code, Codex or any OpenAI client at one base URL. Nothing else in your setup changes.',
		code: `export ANTHROPIC_BASE_URL=${PROXY_URL}\nexport ANTHROPIC_AUTH_TOKEN=[YOUR KEY]\n\n# ~/.codex/config.toml\nmodel_provider = "hara"\nbase_url = "${PROXY_URL}/v1"`,
		file: 'setup',
		id: 'point',
		title: 'Set one',
		token: 'base_url',
	},
	{
		body: 'A conversation and its subagents stay on the account that started it, so prompt caches keep working. At the limit, the next request moves on.',
		code: 'session=7f3a…  auth=claude-personal  model=claude-opus-4-8\nsession=7f3a…  auth=claude-personal  model=claude-opus-4-8\nlimit reached  →  rebinding\nsession=7f3a…  auth=claude-work      model=claude-opus-4-8',
		file: 'routing.log',
		id: 'route',
		title: 'Keep the',
		token: 'session',
	},
	{
		body: 'The proxy runs in Docker on your computer. Devices on your network use hara.local; your other devices reach it over Tailscale, wherever they are.',
		code: `make up\n\n✓ this computer       http://localhost:8317\n✓ your network        ${PROXY_URL}\n✓ your tailnet        https://cliproxy.[TAILNET].ts.net`,
		file: 'make up',
		id: 'reach',
		title: 'Reach it from',
		token: 'anywhere',
	},
];
