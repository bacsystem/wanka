"use client";

import { ArrowLeft, Send } from "lucide-react";
import Link from "next/link";
import { useRouter } from "next/navigation";
import * as React from "react";
import { toast } from "sonner";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Field } from "@/components/shared/field";
import { PageHeader } from "@/components/shared/page-header";
import { SectionCard } from "@/components/shared/section-card";
import { StatusBadge } from "@/components/shared/status-badge";
import { formatCurrency, formatDate, formatDocumentNumber } from "@/lib/format";
import { cn } from "@/lib/utils";
import type { SalesDocumentDetail } from "@/types/domain";
import { documentTypeLabel } from "../lib/document-type";

const types = [{ code: "01", label: "Intereses por mora", text: "Cargo por pago fuera de la fecha de vencimiento pactada." }, { code: "02", label: "Aumento en el valor", text: "Incremento del importe de la operación original (ítems o precio)." }, { code: "03", label: "Penalidades / otros conceptos", text: "Penalidades contractuales u otros cargos adicionales." }];

export function DebitNoteForm({ doc }: { doc: SalesDocumentDetail }) {
  const router = useRouter();
  const [type, setType] = React.useState("01");
  const [amount, setAmount] = React.useState("64.00");
  const [reason, setReason] = React.useState("Intereses moratorios por 12 días de atraso (TEA 12%).");
  const number = formatDocumentNumber(doc.series, doc.number);
  const base = (Number(amount) || 0) / 1.18; const igv = (Number(amount) || 0) - base;
  return (
    <div className="flex min-w-0 flex-1 flex-col gap-4 p-4 md:gap-5 md:p-6">
      <Button variant="ghost" size="sm" className="-ml-2 w-fit" render={<Link href={`/ventas/comprobantes/${doc.id}`} />} nativeButton={false}><ArrowLeft data-icon="inline-start" /> Volver al comprobante</Button>
      <PageHeader eyebrow={`Ventas · Comprobantes · ${number}`} title="Emitir nota de débito electrónica" description="Catálogo 10 SUNAT · serie FD01 · aumenta el importe del comprobante afectado." status={<StatusBadge tone="info" label={`Afecta a ${number}`} />} />
      <div className="grid gap-4 xl:grid-cols-3">
        <div className="flex min-w-0 flex-col gap-4 xl:col-span-2">
          <SectionCard title="Tipo de nota de débito (catálogo 10 SUNAT)" contentClassName="grid gap-2 p-4 sm:grid-cols-3">{types.map((t) => <button key={t.code} type="button" onClick={() => setType(t.code)} aria-pressed={type === t.code} className={cn("flex flex-col gap-1 rounded-md border p-3 text-left hover:border-primary/50", type === t.code && "border-primary bg-accent/40 ring-2 ring-primary/20")}><span className="flex items-center gap-2 text-sm font-medium"><Badge variant="outline" className="font-mono">{t.code}</Badge>{t.label}</span><span className="text-xs text-muted-foreground">{t.text}</span></button>)}</SectionCard>
          <SectionCard title="Detalle del cargo" contentClassName="grid gap-4 p-4 sm:grid-cols-2">
            <Field label="Serie / correlativo"><Input readOnly value="FD01-00000012" className="font-mono" /></Field>
            <Field label="Monto adicional (inc. IGV)"><Input value={amount} onChange={(e) => setAmount(e.target.value)} inputMode="decimal" className="font-mono" /></Field>
            <Field label="Sustento (exigido en el XML)" className="sm:col-span-2"><Textarea rows={3} value={reason} onChange={(e) => setReason(e.target.value)} /></Field>
          </SectionCard>
          <div className="flex justify-end rounded-lg border bg-card p-3"><Button disabled={!(Number(amount) > 0) || reason.trim().length < 10} onClick={() => { toast.success(`Nota de débito FD01-00000012 emitida · ${formatCurrency(Number(amount))}`); router.push(`/ventas/comprobantes/${doc.id}`); }}><Send data-icon="inline-start" /> Emitir y transmitir a SUNAT</Button></div>
        </div>
        <div className="flex min-w-0 flex-col gap-4">
          <SectionCard title="Comprobante afectado" contentClassName="p-4 text-sm"><p className="font-mono text-lg font-semibold">{number}</p><dl className="mt-2 grid gap-1 text-xs">{[["Tipo", `${documentTypeLabel[doc.type]} electrónica`], ["Emisión", formatDate(doc.issuedAt)], ["Cliente", doc.customerName], ["Total original", formatCurrency(doc.total)]].map(([k, v]) => <div key={k} className="flex justify-between gap-2"><dt className="text-muted-foreground">{k}</dt><dd className="text-right font-medium">{v}</dd></div>)}</dl></SectionCard>
          <SectionCard title="Resumen de la nota" contentClassName="p-4 text-sm"><dl className="flex flex-col gap-1"><div className="flex justify-between"><dt className="text-muted-foreground">Op. gravada</dt><dd className="font-mono">{formatCurrency(base)}</dd></div><div className="flex justify-between"><dt className="text-muted-foreground">IGV (18%)</dt><dd className="font-mono">{formatCurrency(igv)}</dd></div><div className="flex items-baseline justify-between border-t pt-2"><dt className="font-semibold">Total nota de débito</dt><dd className="font-mono text-xl font-semibold">{formatCurrency(Number(amount) || 0)}</dd></div><div className="flex justify-between text-xs"><dt className="text-muted-foreground">Nuevo total del comprobante</dt><dd className="font-mono">{formatCurrency(doc.total + (Number(amount) || 0))}</dd></div></dl></SectionCard>
        </div>
      </div>
    </div>
  );
}
