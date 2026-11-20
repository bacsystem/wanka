import type { ShellSession } from "@/components/shell/app-sidebar";
import { tenantOptions } from "@/features/auth/mocks/tenants";

/**
 * Sesión del shell mientras no hay login real: la empresa sale de la cookie de desarrollo `tenant_id` (la que fija el
 * flujo mock de «Seleccionar empresa») y el usuario es un marcador. Se reemplaza por la sesión del BFF en #24 y #27.
 */
export function buildShellSession(tenantId: string | undefined): ShellSession {
  const active = tenantOptions.find((t) => t.id === tenantId) ?? tenantOptions[0];
  return {
    tenants: tenantOptions.map((t) => ({ id: t.id, nombre: t.name, detalle: `RUC ${t.ruc}` })),
    activeTenantId: active.id,
    user: { nombre: "Usuario de prueba", email: "usuario@wanka.pe", rol: active.role },
  };
}
