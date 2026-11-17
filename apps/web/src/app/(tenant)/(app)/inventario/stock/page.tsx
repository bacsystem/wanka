import type { Metadata } from "next";
import { AlertTriangle, Coins, Layers, Truck } from "lucide-react";
import Link from "next/link";
import { StatusBadge } from "@/components/shared/status-badge";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StockScreen } from "@/features/inventory/components/stock-screen";
import { stockCategories, stockMock, stockSummary as s, warehouses } from "@/features/inventory/mocks/stock";
import { formatCurrency, formatInteger } from "@/lib/format";

export const metadata: Metadata = { title: "Stock por almacén" };

export default function StockPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <StockScreen items={stockMock} warehouses={warehouses} categories={stockCategories} kpis={
        <StatGrid>
          <StatCard label="Total ítems / SKU" value={formatInteger(s.skus)} suffix="activos" icon={Layers} footer={<span className="flex items-center gap-2"><span className="size-1.5 rounded-full bg-primary" /><span className="text-primary">{s.available} disponibles</span> • {s.discontinued} descontinuados</span>} />
          <StatCard label="Valorización de almacén" value={formatCurrency(s.valuation)} icon={Coins} tone="info" footer={<span className="flex w-full justify-between"><span>Costo promedio ponderado (SUNAT)</span><span className="font-semibold text-primary">100% conciliado</span></span>} />
          <StatCard label="En stock crítico / mínimo" value={String(s.critical)} icon={AlertTriangle} tone="danger" emphasizeValue hint={<StatusBadge tone="danger" label="Requiere reposición" className="rounded-md" />} footer={<><span className="font-bold text-destructive">!</span> {s.suggestedPO} productos con orden de compra sugerida</>} />
          <StatCard label="Transferencias en tránsito" value={String(s.transfersInTransit)} icon={Truck} tone="info" hint={<span className="font-semibold text-primary">Traslados activos</span>} footer={<span className="flex w-full justify-between"><span>Destino: Sede San Isidro</span><Link href="/ventas/guias" className="font-semibold text-primary hover:underline">Ver GRE ›</Link></span>} />
        </StatGrid>
      } />
    </div>
  );
}
