export function deny(status: number, message: string): Response {
  return Response.json({ error: { type: "permission_error", message } }, { status });
}
