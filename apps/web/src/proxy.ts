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
  // Fuera: la API del BFF, los recursos de Next y los archivos con extensión (favicon, fuentes).
  matcher: ["/((?!api/|_next/|.*\\..*).*)"],
};
