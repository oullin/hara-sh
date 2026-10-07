import { CANONICAL_HOST, DOCS_HOST } from './host.ts';

// Public landing page and documentation share static assets; the proxy runs separately.
type AssetsEnv = {
	ASSETS: { fetch(request: Request): Promise<Response> };
};

function canonicalOriginRedirect(url: URL): Response | undefined {
	if (url.hostname === `www.${CANONICAL_HOST}`) {
		url.hostname = CANONICAL_HOST;
		url.protocol = 'https:';

		return Response.redirect(url.toString(), 301);
	}

	if (url.protocol === 'http:' && (url.hostname === CANONICAL_HOST || url.hostname === DOCS_HOST)) {
		url.protocol = 'https:';

		return Response.redirect(url.toString(), 308);
	}

	return undefined;
}

async function fetchAsset(url: URL, request: Request, env: AssetsEnv): Promise<Response> {
	const response = await env.ASSETS.fetch(new Request(url, request));

	if (response.status !== 404) {
		return response;
	}

	const missing = new Response(response.body, response);

	missing.headers.set('X-Robots-Tag', 'noindex');

	return missing;
}

async function fetchDocsAsset(url: URL, request: Request, env: AssetsEnv): Promise<Response> {
	const response = await fetchAsset(url, request, env);

	const location = response.headers.get('location');

	// Asset filename normalization can redirect (for example @ to %40). Keep
	// the internal /docs prefix out of public redirect URLs.
	if (location !== null) {
		const target = new URL(location, url);

		target.pathname = target.pathname.replace(/^\/docs(?=\/|$)/, '') || '/';

		const redirect = new Response(response.body, response);

		redirect.headers.set('location', target.toString());

		return redirect;
	}

	return response;
}

export default {
	fetch(request: Request, env: AssetsEnv): Response | Promise<Response> {
		const url = new URL(request.url);

		const originRedirect = canonicalOriginRedirect(url);

		if (originRedirect) {
			return originRedirect;
		}

		if (url.hostname === DOCS_HOST) {
			const canonicalPath =
				url.pathname
					.replace(/\/index\.html$/, '/')
					.replace(/\.html$/, '')
					.replace(/\/+$/, '') || '/';

			if (canonicalPath !== url.pathname) {
				url.pathname = canonicalPath;

				return Response.redirect(url.toString(), 308);
			}

			const path = url.pathname === '/' ? '/index.html' : /\/[^/.]+$/.test(url.pathname) ? `${url.pathname}.html` : url.pathname;

			url.pathname = `/docs${path}`;

			return fetchDocsAsset(url, request, env);
		}

		if (url.pathname === '/docs' || url.pathname.startsWith('/docs/')) {
			url.hostname = DOCS_HOST;
			url.pathname = url.pathname.slice('/docs'.length) || '/';

			return Response.redirect(url.toString(), 308);
		}

		if (url.pathname === '/') {
			url.pathname = '/index.html';
		} else if (url.pathname === '/index.html') {
			url.pathname = '/';

			return Response.redirect(url.toString(), 308);
		}

		return fetchAsset(url, request, env);
	},
};
