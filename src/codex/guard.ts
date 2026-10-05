import { deny } from "../http/errors";
import type { Upstream } from "../upstream";
import { CODEX_PATHS, isCodexModel } from "./policy";

// Forwards a request made with the Codex key only if it targets a Codex model.
export async function codexOnly(request: Request, upstream: Upstream): Promise<Response> {
  const { pathname } = new URL(request.url);
  if (!CODEX_PATHS.has(pathname)) return deny(403, `codex key cannot access ${pathname}`);
  // Model names inside a WebSocket stream cannot be checked here; Codex falls back to HTTP.
  if (request.headers.get("upgrade")?.toLowerCase() === "websocket") {
    return deny(403, "codex key cannot use websockets");
  }

  if (pathname === "/v1/models") return listCodexModels(request, upstream);
  if (request.method !== "POST") return deny(405, "method not allowed");

  const text = await request.text();
  let model: unknown;
  try {
    model = (JSON.parse(text) as { model?: unknown }).model;
  } catch {
    return deny(400, "invalid JSON body");
  }
  if (!isCodexModel(model)) {
    return deny(403, `codex key is limited to Codex models; "${String(model)}" is not allowed`);
  }
  return upstream(new Request(request, { body: text }));
}

async function listCodexModels(request: Request, upstream: Upstream): Promise<Response> {
  const res = await upstream(request);
  if (!res.ok) return res;
  const body = (await res.json()) as { data?: { id: string }[] };
  body.data = (body.data ?? []).filter((m) => isCodexModel(m.id));
  return Response.json(body, { status: res.status });
}
