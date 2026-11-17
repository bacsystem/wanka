import type { Metadata } from "next";
import { Calculator, Landmark, Receipt, ShoppingBag, Warehouse } from "lucide-react";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { ReportsScreen } from "@/features/reports/components/reports-screen";
import { reportTypes, reportsSummary as s, salesRegisterMock } from "@/features/reports/mocks/reports";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Reportes SUNAT" };

export default function ReportsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader eyebrow="Contabilidad y finanzas" title="Reportes tributarios y libros electrónicos SUNAT" description={`RUC ${s.ruc} · Registro de ventas (RVIE – SIRE), kardex valorizado y liquidación mensual estimada de IGV / Renta (F-621).`} status={<StatusBadge tone="success" dot label="SIRE API en línea" />} />
      <StatGrid columns={5}>
        <StatCard label="Ventas gravadas" value={formatCurrency(s.taxableSales)} icon={Receipt} hint={`${s.docs} comprobantes emitidos`} />
        <StatCard label="Débito fiscal (18%)" value={formatCurrency(s.fiscalDebit)} icon={Landmark} hint="Impuesto bruto generado" />
        <StatCard label="Crédito fiscal (compras)" value={formatCurrency(s.fiscalCredit)} icon={ShoppingBag} tone="success" hint="Compras e insumos clínicos" />
        <StatCard label="IGV por liquidar" value={formatCurrency(s.igvToPay)} icon={Calculator} tone="warning" hint={`${s.period} abierto · saldo F-621 estimado`} />
        <StatCard label="Kardex valorizado" value={formatCurrency(s.kardex)} icon={Warehouse} hint="Costo promedio · 3 almacenes" />
      </StatGrid>
      <ReportsScreen types={reportTypes} rows={salesRegisterMock} schedule={s.schedule} />
    </div>
  );
}
