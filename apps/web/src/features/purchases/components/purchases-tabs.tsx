import { SectionNav } from "@/components/shared/section-nav";

/** Route tabs shared by the purchases screens (the DS marks the current route). */
export function PurchasesTabs({ counts }: { counts: { registro: number; proveedores: number; ordenes: number } }) {
  return (
    <SectionNav
      aria-label="Secciones de compras"
      items={[
        { href: "/compras", label: "Registro de compras (RCE SIRE)", badge: String(counts.registro) },
        { href: "/compras/proveedores", label: "Proveedores", badge: `${counts.proveedores} activos`, badgeTone: "ok" },
        { href: "/compras/ordenes", label: "Órdenes de compra y detracciones", badge: `${counts.ordenes} pendientes`, badgeTone: "info" },
      ]}
    />
  );
}
