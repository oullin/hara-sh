import { bindings, defineConfig, defineContainer, defineWorker, exports } from "cf/config";

const cliProxy = defineContainer({
	name: "cli-proxy-api",
	// Lifecycle is driven by the CliProxy Durable Object (src/index.ts).
	schedulingPolicy: "durable-object",
	images: {
		default: { dockerfile: "./Dockerfile" },
	},
	observability: { enabled: true, logs: { enabled: true } },
});

const worker = defineWorker({
	name: "cli-proxy-api",
	compatibilityDate: "2026-10-01",
	entrypoint: "src/index.ts",
	workersDev: false,
	domains: ["proxy.hara.sh"],
	observability: { enabled: true },
	env: {
		OBJECTSTORE_ENDPOINT: bindings.secret(),
		OBJECTSTORE_BUCKET: bindings.secret(),
		OBJECTSTORE_ACCESS_KEY: bindings.secret(),
		OBJECTSTORE_SECRET_KEY: bindings.secret(),
		MANAGEMENT_PASSWORD: bindings.secret(),
	},
	exports: {
		CliProxy: exports.durableObject({ storage: "sqlite", container: cliProxy }),
	},
});

export default defineConfig({
	// Personal account "Ollin" (gustavoocanto@gmail.com).
	accountId: "60bada38ab19d58ec34f53af74bfa796",
	worker,
	containers: [cliProxy],
});
