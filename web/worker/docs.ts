// Bundled at build time, so the index matches the Markdown that VitePress renders.
const files = import.meta.glob<string>('@docs/*.md', { eager: true, import: 'default', query: '?raw' });

// Even at 1.5 characters per token, 12,000 characters stay under the model's 8,192-token window.
export const CHUNK_CHARS = 12_000;

export type Chunk = { id: string; text: string; title: string; url: string };

export function chunks(text: string, max = CHUNK_CHARS): Array<string> {
	const parts: Array<string> = [];

	let current = '';

	for (const block of text.split(/\n{2,}/)) {
		for (let start = 0; start < block.length; start += max) {
			const piece = block.slice(start, start + max);

			if (current && current.length + piece.length + 2 > max) {
				parts.push(current);
				current = piece;
			} else {
				current = current ? `${current}\n\n${piece}` : piece;
			}
		}
	}

	if (current) {
		parts.push(current);
	}

	return parts;
}

export function documents(sources: Record<string, string> = files): Array<Chunk> {
	return Object.entries(sources).flatMap(([path, source]) => {
		const slug = path.replace(/^.*\//, '').replace(/\.md$/, '');
		const body = source.replace(/^---\n[\s\S]*?\n---\n/, '').trim();
		const title = /^# (.+)$/m.exec(body)?.[1] ?? slug;
		const url = slug === 'index' ? '/' : `/${slug}`;

		return chunks(body).map((text, n) => ({ id: `${url}#${n}`, text: `title: ${title} | text: ${text}`, title, url }) satisfies Chunk);
	});
}
