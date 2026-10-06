import { Hono } from 'hono';
import { CODEX_PATHS } from '@/codex/policy';
import { forwardCodexRequest, listCodexModels } from '@/codex/program';
import { runProgram } from '@/effect/run';
import { deny } from '@/http/errors';
import { type AppEnv, provideUpstream } from '@/http/env';

// Router for requests made with the Codex key (CODEX_API_KEY). Only the Codex endpoints exist;
// everything else is refused before it reaches the container.
export const codexRoutes = new Hono<AppEnv>();

codexRoutes.use('*', provideUpstream);

// Model names inside a WebSocket stream cannot be checked here; Codex falls back to HTTP.
codexRoutes.use('*', async (c, next) => {
	if (c.req.header('upgrade')?.toLowerCase() === 'websocket') {
		return deny(403, 'codex key cannot use websockets');
	}

	await next();
});

codexRoutes.get('/v1/models', (c) => runProgram(
	listCodexModels(c.req.raw),
	c.get('upstream'),
));
codexRoutes.post('/v1/responses', (c) => runProgram(
	forwardCodexRequest(c.req.raw),
	c.get('upstream'),
));
codexRoutes.post('/v1/chat/completions', (c) => runProgram(
	forwardCodexRequest(c.req.raw),
	c.get('upstream'),
));

codexRoutes.all('*', (c) => (CODEX_PATHS.has(c.req.path) ? deny(405, 'method not allowed') : deny(403, `codex key cannot access ${c.req.path}`)));
