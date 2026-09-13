import type { LucideIcon } from "lucide-react";
import { Activity, Building2, CreditCard, FileText, LayoutDashboard, LifeBuoy, Settings, ShieldCheck, ToggleRight, Users } from "lucide-react";

export interface AdminNavItem { label: string; href: string; icon: LucideIcon; badge?: string; section: "supervision" | "facturacion" | "plataforma" }
export const adminSectionLabel = { supervision: "Supervisión global", facturacion: "Facturación y clientes", plataforma: "Plataforma" } as const;

/** Navigation of the SaaS admin console (separate from the tenant app). */
export const adminNavigation: AdminNavItem[] = [
  { label: "Dashboard central", href: "/admin", icon: LayoutDashboard, section: "supervision" },
  { label: "Empresas (tenants)", href: "/admin/empresas", icon: Building2, badge: "1,482", section: "supervision" },
  { label: "Salud OSE / SUNAT", href: "/admin/monitoreo", icon: Activity, section: "supervision" },
  { label: "Soporte y tickets", href: "/admin/soporte", icon: LifeBuoy, badge: "3", section: "supervision" },
  { label: "Planes y suscripciones", href: "/admin/planes", icon: CreditCard, section: "facturacion" },
  { label: "Facturación del SaaS", href: "/admin/facturacion", icon: FileText, section: "facturacion" },
  { label: "Usuarios de plataforma", href: "/admin/usuarios", icon: Users, section: "plataforma" },
  { label: "Feature flags y módulos", href: "/admin/modulos", icon: ToggleRight, section: "plataforma" },
  { label: "Auditoría y seguridad", href: "/admin/auditoria", icon: ShieldCheck, section: "plataforma" },
  { label: "Configuración núcleo", href: "/admin/configuracion", icon: Settings, section: "plataforma" },
];

export function isAdminActive(pathname: string, href: string) {
  if (href === "/admin") return pathname === "/admin";
  return pathname === href || pathname.startsWith(`${href}/`);
}
