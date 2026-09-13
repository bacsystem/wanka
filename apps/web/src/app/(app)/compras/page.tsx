import type { Metadata } from "next";
import { Landmark, ShieldCheck, ShoppingBag, Wallet } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { PurchasesScreen } from "@/features/purchases/components/purchases-screen";
import { purchasesMock, purchasesSummary as s } from "@/features/purchases/mocks/purchases";
import { suppliersSummary } from "@/features/purchases/mocks/suppliers";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Registro de compras" };

export default function PurchasesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PurchasesScreen purchases={purchasesMock} discrepancy={s.discrepancy} tabCounts={{ registro: s.monthCount, proveedores: suppliersSummary.active, ordenes: suppliersSummary.pendingOrders }} kpis={
        <StatGrid>
          <StatCard label="Total compras del mes" value={formatCurrency(s.monthTotal)} icon={ShoppingBag} footer={<span className="flex w-full justify-between"><span className="flex items-center gap-1"><ShoppingBag className="size-3" /> {s.monthCount} comprobantes</span><span className="font-semibold text-primary">+{s.monthDelta}% vs ago</span></span>} />
          <StatCard label="Crédito fiscal IGV (18%)" value={formatCurrency(s.fiscalCredit)} icon={Landmark} emphasizeValue footer={<span className="flex w-full justify-between"><span>Deducible Renta / Débito</span><span className="rounded bg-accent px-1.5 py-0.5 font-semibold text-primary dark:bg-muted">RCE Anexo 8</span></span>} />
          <StatCard label="Pendiente de pago" value={formatCurrency(s.pendingPay)} icon={Wallet} tone="danger" emphasizeValue footer={<span className="flex w-full justify-between"><span>{s.pendingCount} facturas a crédito</span><span className="font-semibold text-destructive">⏱ {s.dueSoon} por vencer</span></span>} />
          <StatCard label="Conciliación SIRE SUNAT" value={`${s.sireRate}%`} icon={ShieldCheck} tone="info" hint={<span className="font-semibold text-primary">{s.validated} / {s.monthCount} validados</span>} footer={<span className="flex w-full justify-between"><span className="flex items-center gap-1 text-destructive"><span className="size-1.5 rounded-full bg-rose-500" /> {s.discrepancies} discrepancia detectada</span><span className="font-semibold text-primary">Ver detalle</span></span>} />
        </StatGrid>
      } />
    </div>
  );
}
