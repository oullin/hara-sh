import { clientKey, sameKey } from "./auth/client-key";
import { codexOnly } from "./codex/guard";
import { cliProxyUpstream } from "./upstream";

// The Durable Object class must be exported from the Worker's main module.
export { CliProxy } from "./container/cli-proxy";

export default {
  async fetch(request, env, ctx): Promise<Response> {
    const upstream = cliProxyUpstream(ctx);

    if (env.CODEX_API_KEY && sameKey(clientKey(request), env.CODEX_API_KEY)) {
      return codexOnly(request, upstream);
    }
    return upstream(request);
  },
} satisfies ExportedHandler<Env>;
