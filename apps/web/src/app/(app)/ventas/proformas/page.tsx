import type { Metadata } from "next";
import { CheckCircle2, Clock, FileText, Gauge, PlusCircle, SlidersHorizontal } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { QuotesBoard } from "@/features/quotes/components/quotes-board";
import { quotesMock, quotesSummary as s } from "@/features/quotes/mocks/quotes";
import { formatCurrency } from "@/lib/format";

export const metadata: Metadata = { title: "Cotizaciones y proformas" };

export default function QuotesPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader
        eyebrow="Ventas"
        title="Cotizaciones y proformas B2B"
        description="Presupuestos corporativos, seguimiento de aprobación y conversión a comprobante electrónico."
        status={<StatusBadge tone="success" dot label="SUNAT sync OK" />}
        actions={
          <>
            <Button variant="outline"><SlidersHorizontal data-icon="inline-start" /> Configurar lista precios</Button>
            <Button render={<Link href="/ventas/proformas/nueva" />} nativeButton={false}><PlusCircle data-icon="inline-start" /> Nueva cotización</Button>
          </>
        }
      />
      <StatGrid>
        <StatCard label="Cotizaciones activas" value={String(s.active)} icon={FileText} hint={s.activeDelta} footer={<span className="flex w-full justify-between"><span>Valor en cartera</span><strong className="font-mono text-sm text-primary">{formatCurrency(s.portfolio)}</strong></span>} />
        <StatCard label="Aprobadas este mes" value={String(s.approvedMonth)} icon={CheckCircle2} tone="success" hint={<StatusBadge tone="success" label={`${s.approvedRate}% éxito`} />} footer={<span className="flex w-full justify-between"><span>Cerrado facturable</span><strong className="font-mono text-sm text-primary">{formatCurrency(s.approvedAmount)}</strong></span>} />
        <StatCard label="Por vencer (≤ 5 días)" value={String(s.expiring)} icon={Clock} tone="danger" emphasizeValue hint="⚠ Acción urgente" footer={<span className="flex w-full justify-between"><span>En riesgo</span><strong className="font-mono text-sm text-destructive">{formatCurrency(s.expiringAmount)}</strong></span>} />
        <StatCard label="Velocidad cierre" value={String(s.closeDays)} suffix="días prom." icon={Gauge} tone="info" footer={<span className="flex w-full justify-between"><span>Vs mes anterior</span><strong className="text-primary">{s.closeDelta.replace(" vs. mes anterior", " (más ágil)")}</strong></span>} />
      </StatGrid>
      <QuotesBoard quotes={quotesMock} />
    </div>
  );
}
