import { bindings, defineConfig, defineWorker } from 'cf/config';

// Landing page on the apex. worker/index.ts redirects www.hara.sh to hara.sh, answers
// /api/search/* from Workers AI and Vectorize, and serves the static assets Vite built for everything else.
const worker = defineWorker({
	assets: { htmlHandling: 'none', notFoundHandling: 'none', runWorkerFirst: true },
	compatibilityDate: '2026-10-01',
	domains: ['hara.sh', 'www.hara.sh', 'docs.hara.sh'],
	entrypoint: 'worker/index.ts',
	env: {
		AI: bindings.ai(),
		ASSETS: bindings.assets(),
		// Bearer token for POST /api/search/index.
		INDEX_TOKEN: bindings.secret(),
		SEARCH_BUDGET: bindings.rateLimit({ namespace: '831702', simple: { limit: 120, period: 60 } }),
		SEARCH_LIMIT: bindings.rateLimit({ namespace: '831701', simple: { limit: 30, period: 60 } }),
		VECTORIZE: bindings.vectorize({ name: 'hara-docs' }),
	},
	name: 'hara-web',
	observability: { enabled: true },
	workersDev: false,
});

export default defineConfig({
	// Account selection comes from the local cf profile, outside version control.
	worker,
});
