import { defineConfig, defineWorker } from 'cf/config';

// Static landing page: Vite builds the assets, Cloudflare serves them on www.hara.sh.
// The apex (hara.sh) belongs to another service and must not be attached here.
const worker = defineWorker(
	{
		assets: { notFoundHandling: 'single-page-application' },
		compatibilityDate: '2026-10-01',
		domains: ['www.hara.sh'],
		name: 'hara-web',
		observability: { enabled: true },
		workersDev: false,
	},
);

export default defineConfig({
	// Personal account "Ollin".
	accountId: '60bada38ab19d58ec34f53af74bfa796',
	worker,
});
