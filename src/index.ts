import { Container } from "@cloudflare/containers";

const INSTANCE_REGION: DurableObjectLocationHint = "weur";

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

// The Codex client key (CODEX_API_KEY) may only reach Codex/OpenAI models, which the proxy
// serves exclusively from the Codex OAuth accounts. Everything else stays on the main key.
const CODEX_MODEL = /^(gpt-|codex-|o\d)/i;
const CODEX_PATHS = new Set(["/v1/responses", "/v1/chat/completions", "/v1/models"]);

function clientKey(request: Request): string {
  const auth = request.headers.get("authorization") ?? "";
  return auth.toLowerCase().startsWith("bearer ") ? auth.slice(7).trim() : (request.headers.get("x-api-key") ?? "");
}

function sameKey(a: string, b: string): boolean {
  const enc = new TextEncoder();
  const x = enc.encode(a);
  const y = enc.encode(b);
  return x.byteLength === y.byteLength && crypto.subtle.timingSafeEqual(x, y);
}

function deny(status: number, message: string): Response {
  return Response.json({ error: { type: "permission_error", message } }, { status });
}

async function codexOnly(request: Request, upstream: (req: Request) => Promise<Response>): Promise<Response> {
  const { pathname } = new URL(request.url);
  if (!CODEX_PATHS.has(pathname)) return deny(403, `codex key cannot access ${pathname}`);
  // Model names inside a WebSocket stream cannot be checked here; Codex falls back to HTTP.
  if (request.headers.get("upgrade")?.toLowerCase() === "websocket") return deny(403, "codex key cannot use websockets");

  if (pathname === "/v1/models") {
    const res = await upstream(request);
    if (!res.ok) return res;
    const body = (await res.json()) as { data?: { id: string }[] };
    body.data = (body.data ?? []).filter((m) => CODEX_MODEL.test(m.id));
    return Response.json(body, { status: res.status });
  }

  if (request.method !== "POST") return deny(405, "method not allowed");
  const text = await request.text();
  let model: unknown;
  try {
    model = (JSON.parse(text) as { model?: unknown }).model;
  } catch {
    return deny(400, "invalid JSON body");
  }
  if (typeof model !== "string" || !CODEX_MODEL.test(model)) {
    return deny(403, `codex key is limited to Codex models; "${String(model)}" is not allowed`);
  }
  return upstream(new Request(request, { body: text }));
}

export default {
  async fetch(request, env, ctx): Promise<Response> {
    // ctx.exports.CliProxy is the loopback namespace for the sqlite-backed DO
    // declared in cloudflare.config.ts; its generated type is not narrowed yet.
    const namespace = ctx.exports.CliProxy as unknown as DurableObjectNamespace<CliProxy>;
    // One named instance so all requests share the same auth state. Anthropic's OAuth
    // endpoint rejects the default placement (near Dubai) with 403 "Request not allowed",
    // so the DO (and its container) is created with a location hint. Hints only apply when
    // a DO is first created, hence the region in the instance name.
    const id = namespace.idFromName(`main-${INSTANCE_REGION}`);
    const stub = namespace.get(id, { locationHint: INSTANCE_REGION });
    const upstream = (req: Request) => stub.fetch(req);

    if (env.CODEX_API_KEY && sameKey(clientKey(request), env.CODEX_API_KEY)) {
      return codexOnly(request, upstream);
    }
    return upstream(request);
  },
} satisfies ExportedHandler<Env>;
