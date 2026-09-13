import { ArrowRightLeft, Building2, TrendingUp } from "lucide-react";
import { SectionNav } from "@/components/shared/section-nav";

const tabs = [
  { href: "/inventario/almacenes", label: "Almacenes y sedes", icon: Building2, badge: "4" },
  { href: "/inventario/movimientos", label: "Movimientos y traslados", icon: ArrowRightLeft, badge: "465" },
  { href: "/inventario/kardex", label: "Kardex valorizado", icon: TrendingUp, badge: "SUNAT 13.1" },
];

/** Sub-navigation strip shared by the inventory screens. */
export function InventoryTabs({ active }: { active: string }) {
  return <SectionNav aria-label="Secciones de inventario" active={active} items={tabs} />;
}
