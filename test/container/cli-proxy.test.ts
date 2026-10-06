import { describe, expect, it, vi } from 'vitest';

// The real base class imports `cloudflare:workers`, which only exists inside workerd.
vi.mock('@cloudflare/containers', () => ({
	Container: class {
		container: unknown;

		envVars: Record<string, string> = {};

		constructor(ctx: { container: unknown }) {
			this.container = ctx.container;
		}
	},
}));

const { CliProxy } = await import('@/container/cli-proxy');

const env = {
	CODEX_API_KEY: 'codex',
	MANAGEMENT_PASSWORD: 'mgmt',
	OBJECTSTORE_ACCESS_KEY: 'access',
	OBJECTSTORE_BUCKET: 'bucket',
	OBJECTSTORE_ENDPOINT: 'https://r2.test',
	OBJECTSTORE_SECRET_KEY: 'secret',
} as unknown as Env;

function build() {
	const runtime = { images: { default: 'registry/cli-proxy:1' }, start: vi.fn() };
	const proxy = new CliProxy({ container: runtime } as never, env);

	return { proxy, runtime };
}

describe('CliProxy', () => {
	it('serves CLIProxyAPI on 8317 and sleeps after 30 minutes idle', () => {
		const { proxy } = build();

		expect(proxy.defaultPort).toBe(8317);
		expect(proxy.sleepAfter).toBe('30m');
	});

	it('passes the R2 object store and management settings to the container', () => {
		const { proxy } = build();

		expect(proxy.envVars).toEqual({
			MANAGEMENT_PASSWORD: 'mgmt',
			OBJECTSTORE_ACCESS_KEY: 'access',
			OBJECTSTORE_BUCKET: 'bucket',
			OBJECTSTORE_ENDPOINT: 'https://r2.test',
			OBJECTSTORE_SECRET_KEY: 'secret',
		});
		expect(proxy.envVars).not.toHaveProperty('CODEX_API_KEY');
	});

	it('wraps the runtime container so start() uses the explicit image', () => {
		const { proxy, runtime } = build();

		(proxy as unknown as { container: { start: () => void } }).container.start();

		expect(runtime.start).toHaveBeenCalledWith({ enableInternet: true, image: 'registry/cli-proxy:1' });
	});

	it('logs when the container starts', () => {
		const log = vi.spyOn(console, 'log').mockImplementation(() => {});

		build().proxy.onStart();

		expect(log).toHaveBeenCalledWith('cli-proxy-api container started');
	});
});
