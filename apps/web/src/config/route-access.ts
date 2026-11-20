import { isEnabledPath, type Feature, enabledFeatures } from "./navigation";

/**
 * Áreas que NO dependen de la lista de habilitadas: solo el ingreso real (`/login`, #24). Cada ruta cubre su prefijo
 * (`/login`, `/login/x`). Las pantallas mock vuelven cuando su historia las habilita: `/seleccionar-empresa` (#27, #235),
 * `/onboarding` (#31) y la consola `/admin` (su historia); hasta entonces responden 404.
 */
export const OPEN_AREAS: readonly string[] = ["/login"];

/**
 * Prefijos de route handlers del BFF (`/api/...`). Cada historia que agrega una agrega aquí su prefijo (login: #24). Cualquier otro `/api/*` responde 404.
 */
export const OPEN_API_ROUTES: readonly string[] = ["/api/auth/login"];

/** Ruta interna que siempre responde 404 (`app/(tenant)/(app)/[...slug]/page.tsx`); el proxy reescribe a ella lo no habilitado. */
export const DISABLED_ROUTE = "/no-habilitada";

export type RouteAccess = "open" | "enabled" | "disabled";

/**
 * Decide si una URL se atiende. Es por lista blanca: una ruta que no es de ingreso ni está habilitada es `disabled`
 * (404), aunque exista su archivo en `app/`.
 */
export function routeAccess(pathname: string, features: readonly Feature[] = enabledFeatures): RouteAccess {
  if ([...OPEN_AREAS, ...OPEN_API_ROUTES].some((area) => pathname === area || pathname.startsWith(`${area}/`))) return "open";
  return isEnabledPath(pathname, features) ? "enabled" : "disabled";
}
