"use client";

import { ClipboardCheck, ClipboardList, Store } from "lucide-react";
import { SectionNav } from "@/components/shared/section-nav";

export type PurchasesTab = "registro" | "proveedores" | "ordenes";
const href: Record<PurchasesTab, string> = { registro: "/compras", proveedores: "/compras/proveedores", ordenes: "/compras/ordenes" };

/** Pill tabs shared by the purchases screens. */
export function PurchasesTabs({ tab, counts }: { tab: PurchasesTab; counts: { registro: number; proveedores: number; ordenes: number } }) {
  return (
    <SectionNav
      aria-label="Secciones de compras"
      active={href[tab]}
      items={[
        { href: href.registro, label: "Registro de compras (RCE SIRE)", icon: ClipboardList, badge: String(counts.registro) },
        { href: href.proveedores, label: "Proveedores", icon: Store, badge: `${counts.proveedores} activos`, badgeClassName: "bg-success text-success-foreground" },
        { href: href.ordenes, label: "Órdenes de compra y detracciones", icon: ClipboardCheck, badge: `${counts.ordenes} pendientes`, badgeClassName: "bg-info text-info-foreground" },
      ]}
    />
  );
}
