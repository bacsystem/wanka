import type { NextResponse } from "next/server";

/**
 * Cookies de sesión del BFF. Los tokens viven solo aquí: `httpOnly` (el JavaScript del navegador no los lee), `Secure`
 * fuera de local y `SameSite=Lax`. Nunca en `localStorage`. El refresh no se envía a toda la app, solo a `/api/auth`
 * (lo usará el refresh de #234).
 */
export const ACCESS_COOKIE = "wanka_at";
export const REFRESH_COOKIE = "wanka_rt";
/** Token del segundo paso pendiente (`mfa_required`, 5 min): lo canjea la verificación de #25. */
export const CHALLENGE_COOKIE = "wanka_ch";
/** Token del enrolamiento obligatorio de 2FA (`mfa_enrollment_required`): lo usa #25. */
export const ENROLLMENT_COOKIE = "wanka_en";

export const AUTH_COOKIE_PATH = "/api/auth";
const REFRESH_MAX_AGE_S = 30 * 24 * 60 * 60;
const DEFAULT_ACCESS_MAX_AGE_S = 15 * 60;

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

/** Segundos hasta `expiresAt` (RFC 3339); si no se puede leer, los 15 min del acceso. Nunca menos de 1. */
export function secondsUntil(expiresAt: unknown, now: number = Date.now()): number {
  const at = typeof expiresAt === "string" ? Date.parse(expiresAt) : Number.NaN;
  if (Number.isNaN(at)) return DEFAULT_ACCESS_MAX_AGE_S;
  return Math.max(1, Math.floor((at - now) / 1000));
}

/** Pone el par de tokens de una sesión sin empresa (`authenticated`). */
export function setSessionCookies(res: NextResponse, tokens: { accessToken: string; refreshToken: string; expiresAt: string }): void {
  res.cookies.set(ACCESS_COOKIE, tokens.accessToken, baseOptions("/", secondsUntil(tokens.expiresAt)));
  res.cookies.set(REFRESH_COOKIE, tokens.refreshToken, baseOptions(AUTH_COOKIE_PATH, REFRESH_MAX_AGE_S));
}

/** Pone un token de paso intermedio (desafío o enrolamiento de 2FA), solo para las rutas `/api/auth`. */
export function setStepCookie(res: NextResponse, name: typeof CHALLENGE_COOKIE | typeof ENROLLMENT_COOKIE, token: string, expiresAt: string): void {
  res.cookies.set(name, token, baseOptions(AUTH_COOKIE_PATH, secondsUntil(expiresAt)));
}
