import { NextResponse } from "next/server";
import { callApi } from "@/lib/bff/api-client";
import { CHALLENGE_COOKIE, ENROLLMENT_COOKIE, isToken, setSessionCookies, setStepCookie } from "@/lib/bff/session-cookies";
import type { components } from "@/lib/api/schema";
import { validateLogin, type ApiLoginResponse, type LoginError, type LoginOutcome } from "./login-contract";

type LoginRequest = components["schemas"]["LoginRequest"];

const MAX_BODY_BYTES = 4096;
/** Tope de lo que se acepta de `Retry-After`: más allá no es «unos segundos». */
const MAX_RETRY_AFTER_S = 3600;

function respond(body: LoginOutcome | LoginError, status: number, retryAfterSeconds?: number): NextResponse {
  const res = NextResponse.json(body, { status, headers: { "Cache-Control": "no-store" } });
  if (retryAfterSeconds !== undefined) res.headers.set("Retry-After", String(retryAfterSeconds));
  return res;
}

function fail(status: number, error: LoginError): NextResponse {
  return respond(error, status, error.retryAfterSeconds);
}

/** `Retry-After` en segundos enteros o fecha HTTP; `undefined` si falta o no se entiende. */
export function parseRetryAfter(value: string | null, now: number = Date.now()): number | undefined {
  if (!value) return undefined;
  const t = value.trim();
  const seconds = /^\d+$/.test(t) ? Number(t) : Math.ceil((Date.parse(t) - now) / 1000);
  if (!Number.isFinite(seconds)) return undefined;
  return Math.min(Math.max(seconds, 1), MAX_RETRY_AFTER_S);
}

/**
 * Defensa de CSRF para el login (cookie de sesión de otro sitio): el navegador manda `Origin` y `Sec-Fetch-Site` en
 * cualquier POST. Con `Origin`, debe ser el mismo host que la petición; sin él, la petición no puede venir de otro sitio.
 */
export function isSameOrigin(req: Request): boolean {
  const origin = req.headers.get("origin");
  if (origin) {
    const host = req.headers.get("x-forwarded-host") ?? req.headers.get("host");
    try {
      return host !== null && new URL(origin).host === host;
    } catch {
      return false;
    }
  }
  const site = req.headers.get("sec-fetch-site");
  return site === null || site === "same-origin" || site === "none";
}

async function readCredentials(req: Request): Promise<LoginRequest | NextResponse> {
  if (!req.headers.get("content-type")?.toLowerCase().startsWith("application/json")) return fail(400, { code: "malformed_request" });
  let raw: unknown;
  try {
    const text = await req.text();
    if (text.length > MAX_BODY_BYTES) return fail(400, { code: "malformed_request" });
    raw = JSON.parse(text);
  } catch {
    return fail(400, { code: "malformed_request" });
  }
  if (typeof raw !== "object" || raw === null) return fail(400, { code: "malformed_request" });
  const { email, password } = raw as Record<string, unknown>;
  if (typeof email !== "string" || typeof password !== "string" || validateLogin(email, password) !== null) {
    return fail(422, { code: "validation_failed" });
  }
  return { email: email.trim(), password };
}

/** `code` del cuerpo RFC 9457 de la API; `undefined` si no se puede leer. */
async function problemCode(res: Response): Promise<string | undefined> {
  try {
    const body: unknown = await res.json();
    const code = typeof body === "object" && body !== null ? (body as { code?: unknown }).code : undefined;
    return typeof code === "string" ? code : undefined;
  } catch {
    return undefined;
  }
}

/** Qué contesta el navegador a un error de la API. Nunca copia el cuerpo de la API (podría traer detalle interno). */
async function mapApiError(res: Response): Promise<NextResponse> {
  const retry = parseRetryAfter(res.headers.get("retry-after"));
  switch (res.status) {
    case 401:
      return fail(401, { code: "invalid_credentials" });
    case 403:
      return fail(403, { code: "tenant_access_denied" });
    case 400:
    case 422:
      // El BFF ya validó: que la API lo rechace es un desajuste de contrato, no un error del usuario.
      return fail(502, { code: "unavailable" });
    case 429:
      return fail(429, { code: "rate_limited", retryAfterSeconds: retry });
    case 503:
      return fail(503, { code: (await problemCode(res)) === "overloaded" ? "overloaded" : "dependency_unavailable", retryAfterSeconds: retry });
    default:
      return fail(502, { code: "unavailable" });
  }
}

/**
 * `POST /api/auth/login`: llama a `POST /v1/auth/login` y deja los tokens en cookies `httpOnly`. El navegador solo recibe
 * el estado (`authenticated`, `mfa_required`, …). No registra ni la contraseña ni los tokens.
 */
export async function handleLogin(req: Request): Promise<NextResponse> {
  if (!isSameOrigin(req)) return fail(403, { code: "forbidden_origin" });
  const credentials = await readCredentials(req);
  if (credentials instanceof NextResponse) return credentials;

  let upstream: Response;
  try {
    const userAgent = req.headers.get("user-agent");
    upstream = await callApi(req, "/v1/auth/login", { method: "POST", json: credentials, headers: userAgent ? { "User-Agent": userAgent } : undefined });
  } catch (error) {
    // Solo el tipo del error: ni el cuerpo ni la petición.
    console.error("login: API unreachable", error instanceof Error ? error.name : "unknown");
    return fail(502, { code: "unavailable" });
  }
  if (upstream.status !== 200) return await mapApiError(upstream);

  let data: ApiLoginResponse;
  try {
    data = (await upstream.json()) as ApiLoginResponse;
  } catch {
    return fail(502, { code: "unavailable" });
  }

  switch (data.status) {
    case "authenticated": {
      if (!isToken("access", data.accessToken) || !isToken("refresh", data.refreshToken)) return fail(502, { code: "unavailable" });
      const res = respond({ status: "authenticated" }, 200);
      setSessionCookies(res, data);
      return res;
    }
    case "mfa_required": {
      if (!isToken("challenge", data.challengeToken)) return fail(502, { code: "unavailable" });
      const res = respond({ status: "mfa_required" }, 200);
      setStepCookie(res, CHALLENGE_COOKIE, data.challengeToken, data.expiresAt);
      return res;
    }
    case "mfa_enrollment_required": {
      if (!isToken("enrollment", data.enrollmentToken)) return fail(502, { code: "unavailable" });
      const res = respond({ status: "mfa_enrollment_required" }, 200);
      setStepCookie(res, ENROLLMENT_COOKIE, data.enrollmentToken, data.expiresAt);
      return res;
    }
    default:
      return fail(502, { code: "unavailable" });
  }
}
