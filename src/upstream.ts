import type { CliProxy } from "./container/cli-proxy";

export type Upstream = (request: Request) => Promise<Response>;

// Anthropic's OAuth endpoint rejects the default placement (near Dubai) with 403
// "Request not allowed", so the Durable Object (and its container) is created in Western
// Europe. Location hints only apply when a DO is first created, hence the region in the name.
const INSTANCE_REGION: DurableObjectLocationHint = "weur";

// One named instance so every request shares the same auth state.
export function cliProxyUpstream(ctx: ExecutionContext): Upstream {
  // ctx.exports.CliProxy is the loopback namespace for the sqlite-backed DO declared in
  // cloudflare.config.ts; its generated type is not narrowed yet.
  const namespace = ctx.exports.CliProxy as unknown as DurableObjectNamespace<CliProxy>;
  const stub = namespace.get(namespace.idFromName(`main-${INSTANCE_REGION}`), { locationHint: INSTANCE_REGION });
  return (request) => stub.fetch(request);
}
