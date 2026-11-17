import {
  ArrowsLeftRightIcon,
  BankIcon,
  BedIcon,
  BriefcaseIcon,
  BuildingsIcon,
  CalendarBlankIcon,
  ChartBarIcon,
  ChecksIcon,
  ClipboardTextIcon,
  CoinsIcon,
  CookingPotIcon,
  CubeIcon,
  FileTextIcon,
  ForkKnifeIcon,
  GearSixIcon,
  MopedIcon,
  PackageIcon,
  PawPrintIcon,
  PlugsConnectedIcon,
  PlusCircleIcon,
  ReceiptIcon,
  ScissorsIcon,
  ShieldCheckIcon,
  ShoppingBagIcon,
  ShoppingCartIcon,
  SquaresFourIcon,
  StackIcon,
  TagIcon,
  TruckIcon,
  UsersIcon,
  WalletIcon,
  WarehouseIcon,
} from "@phosphor-icons/react/ssr";
import type { IconComponent } from "@/lib/icons";
import type { Industry } from "@/types/domain";

export interface NavItem {
  label: string;
  href: string;
  icon: IconComponent;
  children?: NavItem[];
  /** Sidebar section; defaults to "core". */
  section?: NavSection;
}
export type NavSection = "core" | "operaciones";
export const navSectionLabel: Record<NavSection, string> = { core: "Módulos core", operaciones: "Operaciones" };

/** Single source of truth for app navigation (sidebar, bottom nav, breadcrumbs). */
export const navigation: NavItem[] = [
  { label: "Inicio", href: "/", icon: SquaresFourIcon },
  {
    label: "Ventas",
    href: "/ventas",
    icon: ShoppingCartIcon,
    children: [
      { label: "Nueva venta (POS)", href: "/ventas/nueva", icon: PlusCircleIcon },
      { label: "Comprobantes", href: "/ventas/comprobantes", icon: ReceiptIcon },
      { label: "Cotizaciones y proformas", href: "/ventas/proformas", icon: FileTextIcon },
      { label: "Guías de remisión", href: "/ventas/guias", icon: TruckIcon },
    ],
  },
  { label: "Clientes", href: "/clientes", icon: UsersIcon },
  { label: "Agenda", href: "/agenda", icon: CalendarBlankIcon },
  {
    label: "Catálogo",
    section: "operaciones",
    href: "/catalogo",
    icon: PackageIcon,
    children: [
      { label: "Productos", href: "/catalogo/productos", icon: CubeIcon },
      { label: "Servicios", href: "/catalogo/servicios", icon: BriefcaseIcon },
      { label: "Categorías", href: "/catalogo/categorias", icon: TagIcon },
    ],
  },
  {
    label: "Inventario",
    section: "operaciones",
    href: "/inventario",
    icon: WarehouseIcon,
    children: [
      { label: "Stock por almacén", href: "/inventario/stock", icon: StackIcon },
      { label: "Almacenes", href: "/inventario/almacenes", icon: BuildingsIcon },
      { label: "Movimientos", href: "/inventario/movimientos", icon: ArrowsLeftRightIcon },
      { label: "Kardex", href: "/inventario/kardex", icon: ClipboardTextIcon },
    ],
  },
  {
    label: "Compras",
    section: "operaciones",
    href: "/compras",
    icon: ShoppingBagIcon,
    children: [
      { label: "Registro de compras (SIRE)", href: "/compras", icon: ClipboardTextIcon },
      { label: "Proveedores", href: "/compras/proveedores", icon: BuildingsIcon },
      { label: "Órdenes de compra", href: "/compras/ordenes", icon: ChecksIcon },
    ],
  },
  {
    label: "Finanzas",
    section: "operaciones",
    href: "/finanzas",
    icon: BankIcon,
    children: [
      { label: "Cuentas por cobrar", href: "/finanzas/cuentas-por-cobrar", icon: WalletIcon },
      { label: "Cuentas por pagar", href: "/finanzas/cuentas-por-pagar", icon: BankIcon },
      { label: "Caja y arqueo", href: "/finanzas/caja", icon: CoinsIcon },
      { label: "Historial de turnos", href: "/finanzas/caja/historial", icon: ClipboardTextIcon },
    ],
  },
  {
    label: "Reportes",
    section: "operaciones",
    href: "/reportes",
    icon: ChartBarIcon,
    children: [
      { label: "Gestión comercial", href: "/reportes/gestion", icon: ChartBarIcon },
      { label: "Tributarios SUNAT", href: "/reportes", icon: BankIcon },
      { label: "Liquidación PDT 621", href: "/reportes/liquidacion", icon: ChecksIcon },
    ],
  },
  {
    label: "Configuración",
    section: "operaciones",
    href: "/configuracion",
    icon: GearSixIcon,
    children: [
      { label: "SUNAT y certificado", href: "/configuracion/sunat", icon: ShieldCheckIcon },
      { label: "Usuarios y roles", href: "/configuracion/usuarios", icon: UsersIcon },
      { label: "Integraciones", href: "/configuracion/integraciones", icon: PlugsConnectedIcon },
      { label: "Auditoría", href: "/configuracion/auditoria", icon: ClipboardTextIcon },
    ],
  },
];

/** Industry-specific variations over the base navigation. */
export function getNavigation(industry: Industry): NavItem[] {
  const base = navigation.filter((i) => i.href !== "/agenda");
  const insertAfter = (items: NavItem[], href: string, ...extra: NavItem[]) => { const i = items.findIndex((x) => x.href === href); return [...items.slice(0, i + 1), ...extra, ...items.slice(i + 1)]; };
  switch (industry) {
    case "odontologia":
      return insertAfter(base, "/clientes", { label: "Agenda", href: "/agenda", icon: CalendarBlankIcon });
    case "veterinaria":
      return insertAfter(base.map((i) => (i.href === "/clientes" ? { ...i, label: "Propietarios" } : i)), "/clientes",
        { label: "Mascotas", href: "/mascotas", icon: PawPrintIcon },
        { label: "Agenda", href: "/agenda", icon: CalendarBlankIcon },
        { label: "Hospitalización", href: "/hospitalizacion", icon: BedIcon },
        { label: "Grooming", href: "/grooming", icon: ScissorsIcon });
    case "restaurante":
      return insertAfter(base, "/",
        { label: "Salón y mesas", href: "/salon", icon: ForkKnifeIcon },
        { label: "Comandas / cocina", href: "/salon/cocina", icon: CookingPotIcon },
        { label: "Delivery", href: "/salon/delivery", icon: MopedIcon },
        { label: "Reservas", href: "/salon/reservas", icon: CalendarBlankIcon });
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
