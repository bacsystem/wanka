import type { LucideIcon } from "lucide-react";
import type { Industry } from "@/types/domain";
import {
  ArrowLeftRight,
  BedDouble,
  Bike,
  ChefHat,
  PawPrint,
  UtensilsCrossed,
  BarChart3,
  Box,
  Briefcase,
  Building2,
  CalendarDays,
  ClipboardCheck,
  ClipboardList,
  Coins,
  FileText,
  Landmark,
  Layers,
  LayoutDashboard,
  Package,
  PlusCircle,
  Receipt,
  Scissors,
  Settings,
  ShieldCheck,
  ShoppingBag,
  ShoppingCart,
  Tags,
  Truck,
  Users,
  Wallet,
  Warehouse,
} from "lucide-react";

export interface NavItem {
  label: string;
  href: string;
  icon: LucideIcon;
  children?: NavItem[];
  /** Sidebar section; defaults to "core". */
  section?: NavSection;
}
export type NavSection = "core" | "operaciones";
export const navSectionLabel: Record<NavSection, string> = { core: "Módulos core", operaciones: "Operaciones" };

/** Single source of truth for app navigation (sidebar, bottom nav, breadcrumbs). */
export const navigation: NavItem[] = [
  { label: "Inicio", href: "/", icon: LayoutDashboard },
  {
    label: "Ventas",
    href: "/ventas",
    icon: ShoppingCart,
    children: [
      { label: "Nueva venta (POS)", href: "/ventas/nueva", icon: PlusCircle },
      { label: "Comprobantes", href: "/ventas/comprobantes", icon: Receipt },
      { label: "Cotizaciones y proformas", href: "/ventas/proformas", icon: FileText },
      { label: "Guías de remisión", href: "/ventas/guias", icon: Truck },
    ],
  },
  { label: "Clientes", href: "/clientes", icon: Users },
  { label: "Agenda", href: "/agenda", icon: CalendarDays },
  {
    label: "Catálogo",
    section: "operaciones",
    href: "/catalogo",
    icon: Package,
    children: [
      { label: "Productos", href: "/catalogo/productos", icon: Box },
      { label: "Servicios", href: "/catalogo/servicios", icon: Briefcase },
      { label: "Categorías", href: "/catalogo/categorias", icon: Tags },
    ],
  },
  {
    label: "Inventario",
    section: "operaciones",
    href: "/inventario",
    icon: Warehouse,
    children: [
      { label: "Stock por almacén", href: "/inventario/stock", icon: Layers },
      { label: "Almacenes", href: "/inventario/almacenes", icon: Building2 },
      { label: "Movimientos", href: "/inventario/movimientos", icon: ArrowLeftRight },
      { label: "Kardex", href: "/inventario/kardex", icon: ClipboardList },
    ],
  },
  {
    label: "Compras",
    section: "operaciones",
    href: "/compras",
    icon: ShoppingBag,
    children: [
      { label: "Registro de compras (SIRE)", href: "/compras", icon: ClipboardList },
      { label: "Proveedores", href: "/compras/proveedores", icon: Building2 },
      { label: "Órdenes de compra", href: "/compras/ordenes", icon: ClipboardCheck },
    ],
  },
  {
    label: "Finanzas",
    section: "operaciones",
    href: "/finanzas",
    icon: Landmark,
    children: [
      { label: "Cuentas por cobrar", href: "/finanzas/cuentas-por-cobrar", icon: Wallet },
      { label: "Cuentas por pagar", href: "/finanzas/cuentas-por-pagar", icon: Landmark },
      { label: "Caja y arqueo", href: "/finanzas/caja", icon: Coins },
      { label: "Historial de turnos", href: "/finanzas/caja/historial", icon: ClipboardList },
    ],
  },
  {
    label: "Reportes",
    section: "operaciones",
    href: "/reportes",
    icon: BarChart3,
    children: [
      { label: "Gestión comercial", href: "/reportes/gestion", icon: BarChart3 },
      { label: "Tributarios SUNAT", href: "/reportes", icon: Landmark },
      { label: "Liquidación PDT 621", href: "/reportes/liquidacion", icon: ClipboardCheck },
    ],
  },
  {
    label: "Configuración",
    section: "operaciones",
    href: "/configuracion",
    icon: Settings,
    children: [
      { label: "SUNAT y certificado", href: "/configuracion/sunat", icon: ShieldCheck },
      { label: "Usuarios y roles", href: "/configuracion/usuarios", icon: Users },
      { label: "Integraciones", href: "/configuracion/integraciones", icon: ArrowLeftRight },
      { label: "Auditoría", href: "/configuracion/auditoria", icon: ClipboardList },
    ],
  },
];

/** Industry-specific variations over the base navigation. */
export function getNavigation(industry: Industry): NavItem[] {
  const base = navigation.filter((i) => i.href !== "/agenda");
  const insertAfter = (items: NavItem[], href: string, ...extra: NavItem[]) => { const i = items.findIndex((x) => x.href === href); return [...items.slice(0, i + 1), ...extra, ...items.slice(i + 1)]; };
  switch (industry) {
    case "odontologia":
      return insertAfter(base, "/clientes", { label: "Agenda", href: "/agenda", icon: CalendarDays });
    case "veterinaria":
      return insertAfter(base.map((i) => (i.href === "/clientes" ? { ...i, label: "Propietarios" } : i)), "/clientes",
        { label: "Mascotas", href: "/mascotas", icon: PawPrint },
        { label: "Agenda", href: "/agenda", icon: CalendarDays },
        { label: "Hospitalización", href: "/hospitalizacion", icon: BedDouble },
        { label: "Grooming", href: "/grooming", icon: Scissors });
    case "restaurante":
      return insertAfter(base, "/", 
        { label: "Salón y mesas", href: "/salon", icon: UtensilsCrossed },
        { label: "Comandas / cocina", href: "/salon/cocina", icon: ChefHat },
        { label: "Delivery", href: "/salon/delivery", icon: Bike },
        { label: "Reservas", href: "/salon/reservas", icon: CalendarDays });
    default:
      return base;
  }
}

/** Items shown in the mobile bottom navigation; the rest go under "Más". */
export function getBottomNavHrefs(industry: Industry) {
  if (industry === "restaurante") return ["/", "/salon", "/salon/cocina", "/ventas"];
  if (industry === "veterinaria") return ["/", "/ventas", "/mascotas", "/agenda"];
  return ["/", "/ventas", "/clientes", "/inventario"];
}
export const bottomNavHrefs = getBottomNavHrefs("odontologia");

export function isActivePath(pathname: string, href: string) {
  if (href === "/") return pathname === "/";
  return pathname === href || pathname.startsWith(`${href}/`);
}
