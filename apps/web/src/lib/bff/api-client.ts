import { resolveClientIp } from "./client-ip";

/** Plazo del BFF hacia la API (conventions.md §7: la API responde en 10 s, el BFF espera 12 s, el borde 30 s). */
export const API_TIMEOUT_MS = 12_000;

const API_PATH = /^\/(v1|platform\/v1)\/[A-Za-z0-9._~\-/]*$/;

export interface ApiCallInit {
  method?: string;
  headers?: HeadersInit;
  body?: BodyInit | null;
  /** Cuerpo JSON ya serializado: pone `Content-Type: application/json`. */
  json?: unknown;
}

/**
 * Cabeceras de una llamada a la API. Parte de una lista vacía: NO copia las del navegador, así que un `X-Client-Ip` o un
 * `X-Request-Id` que él mande nunca llegan a la API. Las cabeceras del llamador van primero y las del BFF las pisan.
 */
export function buildApiHeaders(incoming: Headers, extra?: HeadersInit): Headers {
  const headers = new Headers(extra);
  headers.delete("x-client-ip");
  const ip = resolveClientIp(incoming);
  if (ip) headers.set("X-Client-Ip", ip);
  headers.set("X-Request-Id", crypto.randomUUID());
  if (!headers.has("Accept")) headers.set("Accept", "application/json");
  return headers;
}

function apiBaseUrl(): string {
  const base = process.env.WANKA_API_URL;
  if (!base) throw new Error("WANKA_API_URL is not set");
  return base.replace(/\/+$/, "");
}

/**
 * Llama a la API desde un route handler. Solo rutas de la API (`/v1/...` o `/platform/v1/...`); no reintenta (los
 * reintentos son decisión de cada ruta, conventions.md §3) y corta a los 12 s.
 * `incoming` es la petición del navegador: de ella solo sale la IP.
 */
export async function callApi(incoming: Request, path: string, init: ApiCallInit = {}): Promise<Response> {
  if (!API_PATH.test(path) || path.includes("..") || path.includes("//")) throw new Error(`Invalid API path: ${path}`);
  const headers = buildApiHeaders(incoming.headers, init.headers);
  let body = init.body ?? null;
  if (init.json !== undefined) {
    body = JSON.stringify(init.json);
    headers.set("Content-Type", "application/json");
  }
  return fetch(`${apiBaseUrl()}${path}`, {
    method: init.method ?? "GET",
    headers,
    body,
    cache: "no-store",
    signal: AbortSignal.timeout(API_TIMEOUT_MS),
  });
}
