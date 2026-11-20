import { NextResponse, type NextRequest } from "next/server";
import { DISABLED_ROUTE, routeAccess } from "@/config/route-access";

/**
 * Guarda de rutas: lo que no está en la lista de habilitadas (`config/navigation.ts`) responde 404 aunque su
 * pantalla mock siga en `app/`. La URL del navegador no cambia.
 */
export function proxy(request: NextRequest) {
  const { pathname } = request.nextUrl;
  if (routeAccess(pathname) !== "disabled") return NextResponse.next();
  return NextResponse.rewrite(new URL(DISABLED_ROUTE, request.url), { status: 404 });
}

export const config = {
  // Fuera solo los recursos internos de Next y el favicon. Un punto en la URL NO excluye: los segmentos dinámicos
  // aceptan puntos (`/clientes/a.b`) y la lista de habilitadas decide TODA ruta, también `/api/*` (las del BFF se
  // abren en `OPEN_API_ROUTES`).
  matcher: ["/((?!_next/|favicon\\.ico$).*)"],
};
