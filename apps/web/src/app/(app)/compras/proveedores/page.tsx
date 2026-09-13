import type { Metadata } from "next";
import { AlertTriangle, ClipboardCheck, Store, Wallet } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { PurchasesTabs } from "@/features/purchases/components/purchases-tabs";
import { SuppliersPageBody } from "@/features/purchases/components/suppliers-screen";
import { purchasesSummary } from "@/features/purchases/mocks/purchases";
import { suppliersMock, suppliersSummary as s } from "@/features/purchases/mocks/suppliers";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Proveedores" };

export default function SuppliersPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Compras y proveedores" title="Proveedores" description="Directorio de proveedores validado contra el padrón SUNAT, condiciones de pago y cuentas de detracción." status={<StatusBadge tone="success" dot label="Padrón SUNAT en línea" />} />
      <StatGrid>
        <StatCard label="Proveedores activos" value={String(s.active)} icon={Store} hint="Validados con RUC habido" />
        <StatCard label="Observados" value={String(s.observed)} icon={AlertTriangle} tone="warning" hint="Discrepancias SIRE o no habido" />
        <StatCard label="Compras 2026" value={formatCurrency(s.yearTotal)} icon={Wallet} hint="Acumulado enero–setiembre" />
        <StatCard label="Órdenes pendientes" value={String(s.pendingOrders)} icon={ClipboardCheck} hint="Por recepcionar o con detracción pendiente" />
      </StatGrid>
      <PurchasesTabs tab="proveedores" counts={{ registro: purchasesSummary.monthCount, proveedores: s.active, ordenes: s.pendingOrders }} />
      <SuppliersPageBody suppliers={suppliersMock} />
    </div>
  );
}
