import { bindings, defineConfig, defineWorker } from 'cf/config';

// Landing page on the apex. worker/index.ts redirects www.hara.sh to hara.sh and serves the
// static assets Vite built for everything else.
const worker = defineWorker(
	{
		assets: { notFoundHandling: 'none', runWorkerFirst: true },
		compatibilityDate: '2026-10-01',
		domains: ['hara.sh', 'www.hara.sh'],
		entrypoint: 'worker/index.ts',
		env: { ASSETS: bindings.assets() },
		name: 'hara-web',
		observability: { enabled: true },
		workersDev: false,
	},
);

export default defineConfig({
	// Personal account "Ollin".
	accountId: 'YOUR_ACCOUNT_ID',
	worker,
});
