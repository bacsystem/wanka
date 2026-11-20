import { isEnabledPath, type Feature, enabledFeatures } from "./navigation";

/**
 * Áreas que NO dependen de la lista de habilitadas: el flujo de ingreso (mock hasta #24 y #27) y la consola de
 * administración del SaaS, que sigue su propio camino. Cada ruta cubre su prefijo (`/login`, `/login/x`).
 */
export const OPEN_AREAS: readonly string[] = ["/login", "/seleccionar-empresa", "/onboarding", "/admin"];

/** Ruta interna que siempre responde 404 (`app/(tenant)/(app)/[...slug]/page.tsx`); el proxy reescribe a ella lo no habilitado. */
export const DISABLED_ROUTE = "/no-habilitada";

export type RouteAccess = "open" | "enabled" | "disabled";

/**
 * Decide si una URL se atiende. Es por lista blanca: una ruta que no es de ingreso ni está habilitada es `disabled`
 * (404), aunque exista su archivo en `app/`.
 */
export function routeAccess(pathname: string, features: readonly Feature[] = enabledFeatures): RouteAccess {
  if (OPEN_AREAS.some((area) => pathname === area || pathname.startsWith(`${area}/`))) return "open";
  return isEnabledPath(pathname, features) ? "enabled" : "disabled";
}
