export function deny(status: number, message: string): Response {
	return Response.json({ error: { message, type: 'permission_error' } }, { status });
}
