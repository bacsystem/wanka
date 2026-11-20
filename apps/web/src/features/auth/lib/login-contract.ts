import type { components } from "@/lib/api/schema";

/** Respuesta del login de la API (`POST /v1/auth/login`), tipada desde el contrato. */
export type ApiLoginResponse = components["schemas"]["LoginResponse"];

export const EMAIL_MIN = 3;
export const EMAIL_MAX = 254;
export const PASSWORD_MAX = 128;

/** Lo que el BFF le contesta al navegador: nunca trae tokens. */
export type LoginOutcome =
  | { status: "authenticated" }
  | { status: "mfa_required" }
  | { status: "mfa_enrollment_required" };

export type LoginErrorCode =
  | "invalid_credentials"
  | "tenant_access_denied"
  | "rate_limited"
  | "overloaded"
  | "dependency_unavailable"
  | "validation_failed"
  | "malformed_request"
  | "forbidden_origin"
  | "unavailable";

export interface LoginError {
  code: LoginErrorCode;
  /** Segundos que pide esperar la API (`Retry-After`), si los dio. */
  retryAfterSeconds?: number;
}

/** Validación del formulario (la misma del BFF y de la API: `LoginRequest`). Devuelve el campo que falla, o `null`. */
export function validateLogin(email: string, password: string): "email" | "password" | null {
  const e = email.trim();
  if (e.length < EMAIL_MIN || e.length > EMAIL_MAX || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e)) return "email";
  if (password.length < 1 || password.length > PASSWORD_MAX) return "password";
  return null;
}
