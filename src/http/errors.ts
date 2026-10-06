export type ErrorType = 'invalid_request_error' | 'permission_error' | 'upstream_error';

export function errorResponse(status: number, type: ErrorType, message: string): Response {
	return Response.json({ error: { message, type } }, { status });
}

export function deny(status: number, message: string): Response {
	return errorResponse(status, 'permission_error', message);
}
