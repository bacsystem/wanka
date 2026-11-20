import type { NextResponse } from "next/server";

/**
 * Cookies de sesión del BFF (ADR 0004 y conventions.md §2): `wk_at`, `wk_rt` y `wk_at_exp`, todas `httpOnly`, `Secure`
 * fuera de local, `SameSite=Lax` y `Path=/`, porque `proxy.ts` las necesita en cualquier ruta para refrescar antes del
 * render (#234). Nunca en `localStorage`. `wk_at_exp` guarda el vencimiento del acceso (RFC 3339) y vive tanto como el
 * refresh, para que el proxy sepa cuándo refrescar aunque `wk_at` ya haya vencido.
 */
export const ACCESS_COOKIE = "wk_at";
export const REFRESH_COOKIE = "wk_rt";
export const ACCESS_EXP_COOKIE = "wk_at_exp";
/** Token del segundo paso pendiente (`mfa_required`, 5 min): lo canjea la verificación de #25. Solo para `/api/auth`. */
export const CHALLENGE_COOKIE = "wk_ch";
/** Token del enrolamiento obligatorio de 2FA (`mfa_enrollment_required`): lo usa #25. Solo para `/api/auth`. */
export const ENROLLMENT_COOKIE = "wk_en";

export const AUTH_COOKIE_PATH = "/api/auth";
const SESSION_COOKIES = [ACCESS_COOKIE, REFRESH_COOKIE, ACCESS_EXP_COOKIE] as const;
const STEP_COOKIES = [CHALLENGE_COOKIE, ENROLLMENT_COOKIE] as const;
const REFRESH_MAX_AGE_S = 30 * 24 * 60 * 60;

const TOKEN_PATTERNS = {
  access: /^wk_at_[A-Za-z0-9_-]{43}$/,
  refresh: /^wk_rt_[A-Za-z0-9_-]{43}$/,
  challenge: /^wk_ch_[A-Za-z0-9_-]{43}$/,
  enrollment: /^wk_en_[A-Za-z0-9_-]{43}$/,
} as const;

/** Si el valor tiene la forma de token del contrato (`conventions.md` §2): no se guarda en una cookie algo que no lo es. */
export function isToken(kind: keyof typeof TOKEN_PATTERNS, value: unknown): value is string {
  return typeof value === "string" && TOKEN_PATTERNS[kind].test(value);
}

function baseOptions(path: string, maxAge: number) {
  return {
    httpOnly: true,
    // `Secure` en cualquier build de producción (Railway, HTTPS). `next dev` en local va por http y no la acepta.
    secure: process.env.NODE_ENV === "production",
    sameSite: "lax" as const,
    path,
    maxAge,
  };
}

/** TTL máximo de cada token (conventions.md §2): la cookie nunca vive más que el token. */
const MAX_TTL_S = { access: 15 * 60, challenge: 5 * 60, enrollment: 10 * 60 } as const;

/**
 * Segundos de vida de la cookie de un token: hasta `expiresAt` (RFC 3339), sin pasar del TTL del token. `null` si
 * `expiresAt` falta, no se puede leer o ya venció: entonces NO se guarda la cookie (no hay un valor por defecto).
 */
export function cookieTtl(kind: keyof typeof MAX_TTL_S, expiresAt: unknown, now: number = Date.now()): number | null {
  const at = typeof expiresAt === "string" ? Date.parse(expiresAt) : Number.NaN;
  if (Number.isNaN(at)) return null;
  const seconds = Math.floor((at - now) / 1000);
  return seconds >= 1 ? Math.min(seconds, MAX_TTL_S[kind]) : null;
}

/** Pone el par de tokens de una sesión sin empresa (`authenticated`). `accessTtl` sale de `cookieTtl("access", …)`. */
export function setSessionCookies(res: NextResponse, tokens: { accessToken: string; refreshToken: string; expiresAt: string }, accessTtl: number): void {
  res.cookies.set(ACCESS_COOKIE, tokens.accessToken, baseOptions("/", accessTtl));
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, baseOptions("/", REFRESH_MAX_AGE_S));
  res.cookies.set(ACCESS_EXP_COOKIE, tokens.expiresAt, baseOptions("/", REFRESH_MAX_AGE_S));
}

/** Borra las cookies de una sesión anterior (las tres de sesión). */
export function clearSessionCookies(res: NextResponse): void {
  for (const name of SESSION_COOKIES) res.cookies.delete({ name, path: "/" });
}

/** Borra los tokens de paso intermedio de un login anterior (desafío y enrolamiento), salvo `keep`. */
export function clearStepCookies(res: NextResponse, keep?: (typeof STEP_COOKIES)[number]): void {
  for (const name of STEP_COOKIES) if (name !== keep) res.cookies.delete({ name, path: AUTH_COOKIE_PATH });
}

/** Pone un token de paso intermedio (desafío o enrolamiento de 2FA), solo para las rutas `/api/auth`. `ttl` sale de `cookieTtl`. */
export function setStepCookie(res: NextResponse, name: (typeof STEP_COOKIES)[number], token: string, ttl: number): void {
  res.cookies.set(name, token, baseOptions(AUTH_COOKIE_PATH, ttl));
}
