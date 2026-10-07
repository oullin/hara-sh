import { fileURLToPath } from 'node:url';
import { defineConfig, type HeadConfig } from 'vitepress';

const origin = 'https://docs.hara.sh';

export default defineConfig({
	appearance: 'force-dark',
	base: '/',
	cleanUrls: true,
	description: 'Set up, connect and operate your own Hara proxy.',
	head: [
		['link', { href: '/favicon.svg', rel: 'icon' }],
		['link', { href: 'https://fonts.googleapis.com', rel: 'preconnect' }],
		['link', { crossorigin: '', href: 'https://fonts.gstatic.com', rel: 'preconnect' }],
		['link', { href: 'https://fonts.googleapis.com/css2?family=Geist:wght@400;500;600;700&family=Geist+Mono:wght@400;500&display=swap', rel: 'stylesheet' }],
		['meta', { content: '#000000', name: 'theme-color' }],
		// Valid guides override this; VitePress's generated 404 stays unindexed.
		['meta', { content: 'noindex, follow', name: 'robots' }],
		['meta', { content: 'hara', property: 'og:site_name' }],
		['meta', { content: 'en_GB', property: 'og:locale' }],
		['meta', { content: 'https://hara.sh/og.png', property: 'og:image' }],
		['meta', { content: '1200', property: 'og:image:width' }],
		['meta', { content: '630', property: 'og:image:height' }],
		['meta', { content: 'Hara: one centre for every model.', property: 'og:image:alt' }],
		['meta', { content: 'summary_large_image', name: 'twitter:card' }],
		['meta', { content: 'https://hara.sh/og.png', name: 'twitter:image' }],
		['meta', { content: 'Hara: one centre for every model.', name: 'twitter:image:alt' }],
	],
	lang: 'en-GB',
	markdown: { theme: 'github-dark' },
	outDir: fileURLToPath(new URL('../../public/docs', import.meta.url)),
	sitemap: { hostname: origin },
	themeConfig: {
		docFooter: { next: 'Next page', prev: 'Previous page' },
		footer: { message: 'hara · built on CLIProxyAPI' },
		logo: { alt: '', src: '/favicon.svg' },
		logoLink: { link: 'https://hara.sh', target: '_self' },
		nav: [
			{ link: '/', text: 'Overview' },
			{ link: '/setup', text: 'Setup' },
			{ link: 'https://hara.sh', target: '_self', text: 'Landing page' },
		],
		outline: { label: 'On this page', level: [2, 3] },
		search: { provider: 'local' },
		sidebar: [
			{
				items: [
					{ link: '/', text: 'Overview' },
					{ link: '/setup', text: 'Set up the project' },
					{ link: '/providers', text: 'Connect providers' },
					{ link: '/clients', text: 'Configure clients' },
				],
				text: 'Get started',
			},
			{
				items: [
					{ link: '/networking', text: 'Network and TLS' },
					{ link: '/configuration', text: 'Configuration' },
					{ link: '/operations', text: 'Operations' },
					{ link: '/troubleshooting', text: 'Troubleshooting' },
				],
				text: 'Run your proxy',
			},
			{
				items: [
					{ link: '/development', text: 'Website and documentation' },
					{ link: '/privacy', text: 'Privacy and publication' },
				],
				text: 'Contribute',
			},
		],
		siteTitle: 'hara',
	},
	title: 'Hara docs',
	transformPageData(pageData) {
		if (pageData.relativePath === '404.md') {
			pageData.frontmatter.head = [['meta', { content: 'noindex, follow', name: 'robots' }]];

			return;
		}

		const path = pageData.relativePath.replace(/index\.md$/, '').replace(/\.md$/, '');
		const url = `${origin}/${path}`;
		const title = `${pageData.title} | Hara docs`;
		const description = pageData.description;

		const breadcrumbs = [
			{ '@type': 'ListItem', item: 'https://hara.sh/', name: 'Hara', position: 1 },
			{ '@type': 'ListItem', item: `${origin}/`, name: 'Hara docs', position: 2 },
		];

		if (path) {
			breadcrumbs.push({ '@type': 'ListItem', item: url, name: pageData.title, position: 3 });
		}

		const structuredData = {
			'@context': 'https://schema.org',
			'@graph': [
				{
					'@id': `${url}#article`,
					'@type': 'TechArticle',
					description,
					headline: pageData.title,
					image: 'https://hara.sh/og.png',
					inLanguage: 'en-GB',
					isPartOf: { '@type': 'WebSite', name: 'Hara docs', url: `${origin}/` },
					mainEntityOfPage: url,
					url,
				},
				{ '@type': 'BreadcrumbList', itemListElement: breadcrumbs },
			],
		};
		const head: Array<HeadConfig> = [
			['link', { href: url, rel: 'canonical' }],
			['meta', { content: 'index, follow, max-image-preview:large', name: 'robots' }],
			['meta', { content: 'article', property: 'og:type' }],
			['meta', { content: url, property: 'og:url' }],
			['meta', { content: title, property: 'og:title' }],
			['meta', { content: description, property: 'og:description' }],
			['meta', { content: title, name: 'twitter:title' }],
			['meta', { content: description, name: 'twitter:description' }],
			['script', { type: 'application/ld+json' }, JSON.stringify(structuredData).replaceAll('<', String.raw`\u003c`)],
		];

		// Page head data is also managed during client-side navigation.
		pageData.frontmatter.head = [...(pageData.frontmatter.head ?? []), ...head];
	},
});
