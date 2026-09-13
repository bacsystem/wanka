/**
 * Landing module of each navigation group (`/ventas` → `/ventas/comprobantes`).
 * Kept free of React/Lucide imports so `next.config.ts` can load it; `navigation.test.ts`
 * checks every entry is a child of its group in `config/navigation.ts`.
 */
export const groupLandings: Record<string, string> = {
  "/ventas": "/ventas/comprobantes",
  "/inventario": "/inventario/stock",
  "/finanzas": "/finanzas/cuentas-por-cobrar",
  "/configuracion": "/configuracion/sunat",
};

export function groupRedirects() {
  return Object.entries(groupLandings).map(([source, destination]) => ({ source, destination, permanent: false }));
}
