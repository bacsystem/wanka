import type { Metadata } from "next";
import { CheckCircle2, Clock, Download, PlusCircle, Receipt, RefreshCw, XCircle } from "lucide-react";
import Link from "next/link";
import { Button } from "@/components/ui/button";
import { PageHeader } from "@/components/shared/page-header";
import { StatCard, StatGrid } from "@/components/shared/stat-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { DocumentsList } from "@/features/documents/components/documents-list";
import { documentsMock, documentsSummary as s } from "@/features/documents/mocks/documents";
import { formatCurrency, formatInteger } from "@/lib/format";

export const metadata: Metadata = { title: "Comprobantes" };

export default function DocumentsPage() {
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <PageHeader
        eyebrow="Ventas"
        title="Comprobantes de pago electrónicos"
        description="Emisión, seguimiento tributario y control de comprobantes autorizados por SUNAT · RUC 20608941235"
        status={<StatusBadge tone="info" dot label="Sincronización OSE Bizlinks activa" />}
        actions={
          <>
            <Button variant="outline">
              <Download data-icon="inline-start" /> Exportar (Excel/CSV)
            </Button>
            <Button render={<Link href="/ventas/nueva" />} nativeButton={false}>
              <PlusCircle data-icon="inline-start" /> Emitir comprobante
            </Button>
          </>
        }
      />
      <StatGrid>
        <StatCard label="Total emitido (periodo)" value={formatCurrency(s.totalIssued)} icon={Receipt} tone="info" deltaPercent={s.deltaPercent} deltaLabel={`${s.count} comprobantes en setiembre`} />
        <StatCard label="Aceptados SUNAT (CDR)" value={formatInteger(s.accepted)} icon={CheckCircle2} tone="success" footer={<span className="flex w-full items-center justify-between"><StatusBadge tone="success" dot label={`${((s.accepted / s.count) * 100).toFixed(1)}% tasa éxito`} /><span className="font-mono text-foreground">{formatCurrency(s.acceptedAmount)}</span></span>} />
        <StatCard label="Pendientes / en cola OSE" value={formatInteger(s.pending)} icon={Clock} tone="warning" emphasizeValue footer={<span className="flex w-full items-center justify-between"><span>Monto: <strong className="font-mono text-foreground">{formatCurrency(s.pendingAmount)}</strong></span><Link href="/ventas/comprobantes?estado=pendiente" className="flex items-center gap-1 font-semibold text-primary hover:underline"><RefreshCw className="size-3" /> Reenviar lote</Link></span>} />
        <StatCard label="Rechazados / obs." value={formatInteger(s.rejected)} icon={XCircle} tone="danger" emphasizeValue footer={<span className="flex w-full items-center justify-between"><StatusBadge tone="danger" label="Error tributario" /><span className="font-semibold text-destructive">Requiere corrección</span></span>} />
      </StatGrid>
      <DocumentsList documents={documentsMock} />
    </div>
  );
}
