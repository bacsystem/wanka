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
  | "payload_too_large"
  | "malformed_request"
  | "forbidden_origin"
  | "unavailable";

export interface LoginError {
  code: LoginErrorCode;
  /** Segundos que pide esperar la API (`Retry-After`), si los dio. */
  retryAfterSeconds?: number;
}

/** Largo en caracteres (puntos de código), como lo mide la API (`utf8.RuneCountInString`) y JSON Schema; no en unidades UTF-16. */
export function runeLength(value: string): number {
  return [...value].length;
}

/** Validación del formulario (la misma del BFF y de la API: `LoginRequest`). Devuelve el campo que falla, o `null`. */
export function validateLogin(email: string, password: string): "email" | "password" | "passwordTooLong" | null {
  const e = email.trim();
  const emailLength = runeLength(e);
  if (emailLength < EMAIL_MIN || emailLength > EMAIL_MAX || !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(e)) return "email";
  const passwordLength = runeLength(password);
  if (passwordLength < 1) return "password";
  if (passwordLength > PASSWORD_MAX) return "passwordTooLong";
  return null;
}
