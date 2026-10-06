import { CANONICAL_HOST } from './host.ts';

// Front Worker for the landing page: www.hara.sh redirects permanently to the apex, and every
// other request is served from the static assets Vite built.
type AssetsEnv = {
	ASSETS: { fetch(request: Request): Promise<Response> };
};

export default {
	fetch(request: Request, env: AssetsEnv): Promise<Response> | Response {
		const url = new URL(request.url);

		if (url.hostname === `www.${CANONICAL_HOST}`) {
			url.hostname = CANONICAL_HOST;

			return Response.redirect(url.toString(), 301);
		}

		return env.ASSETS.fetch(request);
	},
};
