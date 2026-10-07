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

// Default Docker host endpoint.
export const PROXY_URL = 'http://localhost:8317';

export const DOCS_URL = 'https://docs.hara.sh/';

export type Client = { command: string; id: string; label: string };

// The hero's "For …" switch: the one line each client needs.
export const clients: Array<Client> = [
	{ command: `export ANTHROPIC_BASE_URL=${PROXY_URL}`, id: 'claude', label: 'For Claude Code' },
	{ command: `base_url = "${PROXY_URL}/v1"`, id: 'codex', label: 'For Codex' },
	{ command: `curl -H "Authorization: Bearer YOUR_KEY" ${PROXY_URL}/v1/models`, id: 'openai', label: 'For any client' },
];

export type Step = {
	body: string;
	code: string;
	file: string;
	id: string;
	title: string;
	token: string;
};

// Each setup step drives the adjacent code example.
export const steps: Array<Step> = [
	{
		body: 'Docker builds the tools, generates private keys, and starts the proxy. No host Go installation or 1Password account needed.',
		code: 'docker compose -f local/compose.yaml up -d --build --force-recreate',
		file: 'Terminal',
		id: 'connect',
		title: 'Start with',
		token: 'Docker',
	},
	{
		body: 'Open the management panel and connect your provider accounts. Credentials stay in your private state directory.',
		code: 'http://localhost:8317/management.html\n\n# Get your management password.\ndocker compose -f local/compose.yaml run --rm --no-deps -T tools key management-password',
		file: 'Management panel',
		id: 'point',
		title: 'Connect your',
		token: 'accounts',
	},
	{
		body: 'Set the URL and client key in Claude Code, Codex, or any OpenAI-compatible client. Keep your existing workflow.',
		code: `# Claude Code\nexport ANTHROPIC_BASE_URL=${PROXY_URL}\nexport ANTHROPIC_AUTH_TOKEN="YOUR_KEY"\n\n# OpenAI-compatible clients\nbase_url = "${PROXY_URL}/v1"`,
		file: 'Client settings',
		id: 'route',
		title: 'Point your',
		token: 'client',
	},
	{
		body: 'Use the loopback endpoint on your computer, a stable local domain through portless, or Tailscale for private HTTPS from another device.',
		code: 'docker compose -f local/compose.yaml --profile tailscale up -d --build --force-recreate\n\n# Host client URL\nhttp://localhost:8317\n\n# Local domain (portless)\nhttps://hara.local\n\n# Remote client URL\nhttps://cliproxy.YOUR-TAILNET.ts.net',
		file: 'Optional remote access',
		id: 'reach',
		title: 'Use it',
		token: 'anywhere',
	},
];
