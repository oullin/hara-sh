// The key a client sent, from `Authorization: Bearer <key>` or `x-api-key: <key>`.
export function clientKey(request: Request): string {
	const auth = request.headers.get('authorization') ?? '';

	if (auth.toLowerCase().startsWith('bearer ')) {
		return auth.slice(7).trim();
	}

	return request.headers.get('x-api-key') ?? '';
}

export function sameKey(a: string, b: string): boolean {
	const enc = new TextEncoder();
	const x = enc.encode(a);
	const y = enc.encode(b);

	return x.byteLength === y.byteLength && crypto.subtle.timingSafeEqual(x, y);
}
