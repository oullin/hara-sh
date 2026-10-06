import { bindings, defineConfig, defineContainer, defineWorker, exports } from 'cf/config';

const cliProxy = defineContainer({
	images: {
		default: { dockerfile: './Dockerfile' },
	},
	name: 'cli-proxy-api',
	observability: { enabled: true, logs: { enabled: true } },
	// Lifecycle is driven by the CliProxy Durable Object (src/container/cli-proxy.ts).
	schedulingPolicy: 'durable-object',
});

const worker = defineWorker({
	compatibilityDate: '2026-10-01',
	domains: ['proxy.hara.sh'],
	entrypoint: 'src/index.ts',
	env: {
		// Client key restricted to Codex models by the Worker (src/codex/guard.ts).
		CODEX_API_KEY: bindings.secret(),
		MANAGEMENT_PASSWORD: bindings.secret(),
		OBJECTSTORE_ACCESS_KEY: bindings.secret(),
		OBJECTSTORE_BUCKET: bindings.secret(),
		OBJECTSTORE_ENDPOINT: bindings.secret(),
		OBJECTSTORE_SECRET_KEY: bindings.secret(),
	},
	exports: {
		CliProxy: exports.durableObject({ container: cliProxy, storage: 'sqlite' }),
	},
	name: 'cli-proxy-api',
	observability: { enabled: true },
	workersDev: false,
});

export default defineConfig({
	// Personal account "Ollin" (gustavoocanto@gmail.com).
	accountId: '60bada38ab19d58ec34f53af74bfa796',
	containers: [cliProxy],
	worker,
});
