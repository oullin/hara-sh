import { createMiddleware } from 'hono/factory';
import { cliProxyUpstream, type Upstream } from '@/upstream';

// Hono bindings and per-request variables shared by every router.
export type AppEnv = {
	Bindings: Env;
	Variables: { upstream: Upstream['Service'] };
};

// Gives each request the Upstream service for its execution context.
export const provideUpstream = createMiddleware<AppEnv>(async (c, next) => {
	c.set('upstream', cliProxyUpstream(c.executionCtx as ExecutionContext));

	await next();
});
