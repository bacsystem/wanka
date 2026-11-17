import {
  BuildingsIcon,
  CreditCardIcon,
  FileTextIcon,
  GearSixIcon,
  LifebuoyIcon,
  PulseIcon,
  ShieldCheckIcon,
  SquaresFourIcon,
  ToggleRightIcon,
  UsersIcon,
} from "@phosphor-icons/react/ssr";
import type { IconoComponente } from "@/lib/iconos";

export interface AdminNavItem { label: string; href: string; icon: IconoComponente; badge?: string; section: "supervision" | "facturacion" | "plataforma" }
export const adminSectionLabel = { supervision: "Supervisión global", facturacion: "Facturación y clientes", plataforma: "Plataforma" } as const;

/** Navigation of the SaaS admin console (separate from the tenant app). */
export const adminNavigation: AdminNavItem[] = [
  { label: "Dashboard central", href: "/admin", icon: SquaresFourIcon, section: "supervision" },
  { label: "Empresas (tenants)", href: "/admin/empresas", icon: BuildingsIcon, badge: "1,482", section: "supervision" },
  { label: "Salud OSE / SUNAT", href: "/admin/monitoreo", icon: PulseIcon, section: "supervision" },
  { label: "Soporte y tickets", href: "/admin/soporte", icon: LifebuoyIcon, badge: "3", section: "supervision" },
  { label: "Planes y suscripciones", href: "/admin/planes", icon: CreditCardIcon, section: "facturacion" },
  { label: "Facturación del SaaS", href: "/admin/facturacion", icon: FileTextIcon, section: "facturacion" },
  { label: "Usuarios de plataforma", href: "/admin/usuarios", icon: UsersIcon, section: "plataforma" },
  { label: "Feature flags y módulos", href: "/admin/modulos", icon: ToggleRightIcon, section: "plataforma" },
  { label: "Auditoría y seguridad", href: "/admin/auditoria", icon: ShieldCheckIcon, section: "plataforma" },
  { label: "Configuración núcleo", href: "/admin/configuracion", icon: GearSixIcon, section: "plataforma" },
];

export function isAdminActive(pathname: string, href: string) {
  if (href === "/admin") return pathname === "/admin";
  return pathname === href || pathname.startsWith(`${href}/`);
}
