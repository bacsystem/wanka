/** Usuarios de prueba del login (E2E contra la API real). Uno por proyecto de Playwright: los tres corren a la vez y cada fallo cuenta contra el bloqueo de SU cuenta. */
export type LoginUserKind = "ok" | "bad" | "nomembership" | "mfa";

export const E2E_DOMAIN = "e2e.wanka.test";

export function e2eEmail(kind: LoginUserKind, project: string): string {
  return `e2e-${kind}-${project}@${E2E_DOMAIN}`;
}

export const E2E_PROJECTS = ["desktop", "tablet", "mobile"] as const;
export const E2E_KINDS: readonly LoginUserKind[] = ["ok", "bad", "nomembership", "mfa"];
