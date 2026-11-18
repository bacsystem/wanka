import { SectionNav } from "@/components/shared/section-nav";

const tabs = [
  { href: "/inventario/almacenes", label: "Almacenes y sedes", badge: "4" },
  { href: "/inventario/movimientos", label: "Movimientos y traslados", badge: "465" },
  { href: "/inventario/kardex", label: "Kardex valorizado", badge: "SUNAT 13.1" },
];

/** Sub-navigation strip shared by the inventory screens (the DS marks the current route). */
export function InventoryTabs() {
  return <SectionNav aria-label="Secciones de inventario" items={tabs} />;
}
