import { Hono } from 'hono';
import { clientKey, sameKey } from '@/auth/client-key';
import { codexRoutes } from '@/codex/routes';
import { runProgram } from '@/effect/run';
import { type AppEnv, provideUpstream } from '@/http/env';
import { Upstream } from '@/upstream';

export const app = new Hono<AppEnv>();

app.use('*', provideUpstream);

// Requests made with the Codex key go through the Codex router; everything else
// (the main key, the management panel, health checks) is forwarded untouched.
app.all('*', (c) => {
	if (c.env.CODEX_API_KEY && sameKey(
		clientKey(c.req.raw),
		c.env.CODEX_API_KEY,
	)) {
		return codexRoutes.fetch(c.req.raw, c.env, c.executionCtx);
	}

	return runProgram(
		Upstream.use((upstream) => upstream.forward(c.req.raw)),
		c.get('upstream'),
	);
});
