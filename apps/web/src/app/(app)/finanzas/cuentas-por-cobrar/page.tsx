import type { Metadata } from "next";
import { AlertTriangle, CheckCircle2, Clock, TrendingUp, Wallet } from "lucide-react";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { ReceivablesScreen } from "@/features/receivables/components/receivables-screen";
import { receivablesMock, receivablesSummary as s } from "@/features/receivables/mocks/receivables";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Cuentas por cobrar" };

export default function ReceivablesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <ReceivablesScreen rows={receivablesMock} weekly={s.weekly} spot={s.spot} kpis={<StatGrid>
        <StatCard label="Total cartera por cobrar" value={formatCurrency(s.portfolio)} icon={Wallet} footer={<span className="flex w-full justify-between"><span className="text-primary">{s.invoices} facturas al crédito</span><span className="rounded bg-muted px-1.5 py-0.5">Línea activa S/ 150k</span></span>} />
        <StatCard label="Vencido (+30 a 90 días)" value={formatCurrency(s.overdue)} icon={AlertTriangle} tone="danger" emphasizeValue footer={<span className="flex w-full justify-between"><span className="flex items-center gap-1 text-destructive"><span className="size-1.5 rounded-full bg-rose-500" /> {s.overdueCount} comprobantes críticos</span><span className="rounded bg-rose-100 px-1.5 py-0.5 text-rose-800 dark:bg-rose-950 dark:text-rose-300">Mora activa</span></span>} />
        <StatCard label="Por vencer (próximos 7 días)" value={formatCurrency(s.dueSoon)} icon={Clock} tone="info" emphasizeValue footer={<span className="flex w-full justify-between"><span>{s.dueSoonCount} vencimientos previstos</span><span className="rounded bg-blue-50 px-1.5 py-0.5 text-blue-800 dark:bg-blue-950/60 dark:text-blue-300">Cobro prioritario</span></span>} />
        <StatCard label="Cobrado en el mes" value={formatCurrency(s.collected)} icon={CheckCircle2} tone="success" emphasizeValue footer={<span className="flex w-full justify-between"><span className="flex items-center gap-1 text-primary"><TrendingUp className="size-3" /> {s.recovery}% recuperación</span><span>Meta: {formatCurrency(s.goal)}</span></span>} />
      </StatGrid>} />
    </div>
  );
}
