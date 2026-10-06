import { Container } from '@cloudflare/containers';
import { withExplicitImage } from '@/container/explicit-image';

// Durable Object that owns the CLIProxyAPI container (eceasy/cli-proxy-api, see Dockerfile).
// `Env` is generated from cloudflare.config.ts by `cf workers types`.
export class CliProxy extends Container<Env> {
	defaultPort = 8317;
	sleepAfter = '30m';

	constructor(ctx: ConstructorParameters<typeof Container<Env>>[0], env: Env) {
		super(ctx, env);
		// `globalThis.Container` is the runtime container type; `Container` here is the library class.
		(this as unknown as { container: globalThis.Container }).container = withExplicitImage(ctx.container!);

		// Config and OAuth tokens live in R2 through the server's object store.
		this.envVars = {
			MANAGEMENT_PASSWORD: env.MANAGEMENT_PASSWORD,
			OBJECTSTORE_ACCESS_KEY: env.OBJECTSTORE_ACCESS_KEY,
			OBJECTSTORE_BUCKET: env.OBJECTSTORE_BUCKET,
			OBJECTSTORE_ENDPOINT: env.OBJECTSTORE_ENDPOINT,
			OBJECTSTORE_SECRET_KEY: env.OBJECTSTORE_SECRET_KEY,
		};
	}

	override onStart() {
		// oxlint-disable-next-line no-console -- intentional: container lifecycle in Workers logs
		console.log('cli-proxy-api container started');
	}
}
