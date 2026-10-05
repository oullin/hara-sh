import { Container, getContainer } from "@cloudflare/containers";

interface Env {
  CLI_PROXY: DurableObjectNamespace<CliProxy>;
  OBJECTSTORE_ENDPOINT: string;
  OBJECTSTORE_BUCKET: string;
  OBJECTSTORE_ACCESS_KEY: string;
  OBJECTSTORE_SECRET_KEY: string;
  MANAGEMENT_PASSWORD: string;
}

export class CliProxy extends Container<Env> {
  defaultPort = 8317;
  sleepAfter = "30m";

  constructor(ctx: DurableObjectState<{}>, env: Env) {
    super(ctx, env);
    this.envVars = {
      OBJECTSTORE_ENDPOINT: env.OBJECTSTORE_ENDPOINT,
      OBJECTSTORE_BUCKET: env.OBJECTSTORE_BUCKET,
      OBJECTSTORE_ACCESS_KEY: env.OBJECTSTORE_ACCESS_KEY,
      OBJECTSTORE_SECRET_KEY: env.OBJECTSTORE_SECRET_KEY,
      MANAGEMENT_PASSWORD: env.MANAGEMENT_PASSWORD,
    };
  }

  override onStart() {
    console.log("cli-proxy-api container started");
  }

  override onError(error: unknown) {
    console.error("cli-proxy-api container error", error);
    throw error;
  }
}

export default {
  async fetch(request: Request, env: Env): Promise<Response> {
    // One named instance so all requests share the same auth state.
    return getContainer(env.CLI_PROXY, "main").fetch(request);
  },
} satisfies ExportedHandler<Env>;
