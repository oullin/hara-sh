import { Container, getContainer } from "@cloudflare/containers";

// `Env` is generated from cloudflare.config.ts by `cf workers types`.
export class CliProxy extends Container<Env> {
  defaultPort = 8317;
  sleepAfter = "30m";

  constructor(ctx: DurableObjectState<{}>, env: Env) {
    super(ctx, env);
    // Durable Object-scheduled containers (cf `schedulingPolicy: "durable-object"`) must be
    // started with an explicit image, but @cloudflare/containers 0.3.7 calls start() without
    // one. Inject the "default" image from cloudflare.config.ts until the library supports it.
    const runtime = ctx.container!;
    (this as unknown as { container: globalThis.Container }).container = new Proxy(runtime, {
      get(target, prop) {
        if (prop === "start") {
          return (options?: Omit<ContainerStartupOptions, "image" | "containerSnapshot">) =>
            target.start({ enableInternet: true, ...options, image: target.images.default });
        }
        const value = Reflect.get(target, prop, target);
        return typeof value === "function" ? value.bind(target) : value;
      },
    });
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
}

export default {
  async fetch(request, _env, ctx): Promise<Response> {
    // ctx.exports.CliProxy is the loopback namespace for the sqlite-backed DO
    // declared in cloudflare.config.ts; its generated type is not narrowed yet.
    const namespace = ctx.exports.CliProxy as unknown as DurableObjectNamespace<CliProxy>;
    // One named instance so all requests share the same auth state.
    return getContainer(namespace, "main").fetch(request);
  },
} satisfies ExportedHandler<Env>;
